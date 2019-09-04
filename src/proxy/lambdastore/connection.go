package lambdastore

import (
	"errors"
	"fmt"
	"github.com/wangaoone/LambdaObjectstore/lib/logger"
	"github.com/wangaoone/redeo/resp"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wangaoone/LambdaObjectstore/src/proxy/collector"
	"github.com/wangaoone/LambdaObjectstore/src/proxy/global"
	"github.com/wangaoone/LambdaObjectstore/src/proxy/types"
)

var (
	defaultConnectionLog = &logger.ColorLogger{
		Prefix: fmt.Sprintf("Undesignated "),
		Color:  true,
	}
)

type Connection struct {
	instance *Instance
	log      logger.ILogger
	cn       net.Conn
	w        *resp.RequestWriter
	r        resp.ResponseReader
	mu       sync.Mutex
	respType chan interface{}
	closed   chan struct{}
}

func NewConnection(c net.Conn) *Connection {
	conn := &Connection{
		log: defaultConnectionLog,
		cn:  c,
		// wrap writer and reader
		w:      resp.NewRequestWriter(c),
		r:      resp.NewResponseReader(c),
		respType: make(chan interface{}),
		closed: make(chan struct{}),
	}
	defaultConnectionLog.Level = global.Log.GetLevel()
	return conn
}

func (conn *Connection) GraceClose() {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	select {
	case <-conn.closed:
		// already closed
		return
	default:
	}

	// Signal colosed only. This allow ongoing transmission to finish.
	close(conn.closed)
}

func (conn *Connection) Close() {
	conn.GraceClose()
	// Don't use c.Close(), it will stuck and wait for lambda.
	conn.cn.(*net.TCPConn).SetLinger(0) // The operating system discards any unsent or unacknowledged data.
	conn.cn.Close()
}

// blocking on lambda peek Type
// lambda handle incoming lambda store response
//
// field 0 : conn id
// field 1 : req id
// field 2 : chunk id
// field 3 : obj val
func (conn *Connection) ServeLambda() {
	for {
		go conn.peekResponse()

		var retPeek interface{}
		select {
		case <-conn.closed:
			conn.Close()
			return
		case retPeek = <-conn.respType:
		}

		var respType resp.ResponseType
		switch retPeek.(type) {
		case error:
			err := retPeek.(error)
			if err == io.EOF {
				conn.log.Warn("Lambda store disconnected.")
			} else {
				conn.log.Warn("Failed to peek response type: %v", err)
			}
			conn.Close()
			return
		case resp.ResponseType:
			respType = retPeek.(resp.ResponseType)
		}

		start := time.Now()
		switch respType {
		case resp.TypeError:
			strErr, err := conn.r.ReadError()
			if err != nil {
				err = errors.New(fmt.Sprintf("Response error (Unknown): %v", err))
			} else {
				err = errors.New(fmt.Sprintf("Response error: %s", strErr))
			}
			conn.log.Warn("%v", err)
			conn.instance.SetErrorResponse(err)
		default:
			cmd, err := conn.r.ReadBulkString()
			if err != nil && err == io.EOF {
				conn.log.Warn("Lambda store disconnected.")
				conn.Close()
			} else if err != nil {
				conn.log.Warn("Error on read response type: %v", err)
				break
			}

			switch cmd {
			case "pong":
				conn.pongHandler()
			case "get":
				conn.getHandler(start)
			case "set":
				conn.setHandler(start)
			case "data":
				conn.receiveData()
			case "initMigrate":
				go conn.initMigrateHandler()
			default:
				conn.log.Warn("Unsupported response type: %s", cmd)
			}
		}

		if conn.instance != nil {
			conn.instance.warmUp()
		}
	}
}

func (conn *Connection) Ping() {
	conn.w.WriteCmdString("ping")
	err := conn.w.Flush()
	if err != nil {
		conn.log.Warn("Flush pipeline error(ping): %v", err)
	}
}

func (conn *Connection) peekResponse() {
	respType, err := conn.r.PeekType()
	if err != nil {
		conn.respType <- err
	} else {
		conn.respType <- respType
	}
}

func (conn *Connection) pongHandler() {
	conn.log.Debug("PONG from lambda.")

	// Read lambdaId
	id, _ := conn.r.ReadInt()

	if conn.instance != nil {
		conn.instance.flagValidated(conn)
		return
	}

	// Lock up lambda instance
	instance, exists := Registry.Instance(uint64(id))
	if !exists {
		conn.log.Error("Failed to match lambda: %d", id)
		return
	}
	instance.flagValidated(conn)
	conn.log.Debug("PONG from lambda confirmed.")
}

func (conn *Connection) getHandler(start time.Time) {
	conn.log.Debug("GET from lambda.")

	// Exhaust all values to keep protocol aligned.
	connId, _ := conn.r.ReadBulkString()
	reqId, _ := conn.r.ReadBulkString()
	chunkId, _ := conn.r.ReadBulkString()
	counter, ok := global.ReqMap.Get(reqId)
	if ok == false {
		conn.log.Warn("Request not found: %s", reqId)
		// exhaust value field
		if err := conn.r.SkipBulk(); err != nil {
			conn.log.Warn("Failed to skip bulk on request mismatch: %v", err)
		}
		return
	}

	rsp := &types.Response{Cmd: "get"}
	rsp.Id.ConnId, _ = strconv.Atoi(connId)
	rsp.Id.ReqId = reqId
	rsp.Id.ChunkId = chunkId

	abandon := false
	reqCounter := atomic.AddInt32(&(counter.(*types.ClientReqCounter).Counter), 1)
	// Check if chunks are enough? Shortcut response if YES.
	if int(reqCounter) > counter.(*types.ClientReqCounter).DataShards {
		abandon = true
		conn.log.Debug("GOT %v, abandon.", rsp.Id)
		conn.instance.SetResponse(rsp)
		if err := collector.Collect(collector.LogProxy, rsp.Cmd, rsp.Id.ReqId, rsp.Id.ChunkId, start.UnixNano(), int64(time.Since(start)), int64(0)); err != nil {
			conn.log.Warn("LogProxy err %v", err)
		}
	}

	// Read value
	// t9 := time.Now()
	// bodyStream, err := conn.r.ReadBulk(nil)
	// time9 := time.Since(t9)
	// if err != nil {
	// 	conn.log.Warn("Failed to read value of response: %v", err)
	// 	// Abandon errant data
	// 	res = nil
	// }
	// Skip on abandon
	if abandon {
		if err := conn.r.SkipBulk(); err != nil {
			conn.log.Warn("Failed to skip bulk on abandon: %v", err)
		}
		return
	}

	var err error
	rsp.BodyStream, err = conn.r.StreamBulk()
	if err != nil {
		conn.log.Warn("Failed to get body reader of response: %v", err)
	}
	rsp.BodyStream.(resp.Holdable).Hold()
	defer rsp.BodyStream.Close()

	conn.log.Debug("GOT %v, confirmed.", rsp.Id)
	if !conn.instance.SetResponse(rsp) {
		// Failed to set response, release hold.
		rsp.BodyStream.(resp.Holdable).Unhold()
	}
	if err := collector.Collect(collector.LogProxy, rsp.Cmd, rsp.Id.ReqId, rsp.Id.ChunkId, start.UnixNano(), int64(time.Since(start)), int64(0)); err != nil {
		conn.log.Warn("LogProxy err %v", err)
	}
}

func (conn *Connection) setHandler(start time.Time) {
	conn.log.Debug("SET from lambda.")

	rsp := &types.Response{Cmd: "set", Body: []byte{1}}
	connId, _ := conn.r.ReadBulkString()
	rsp.Id.ConnId, _ = strconv.Atoi(connId)
	rsp.Id.ReqId, _ = conn.r.ReadBulkString()
	rsp.Id.ChunkId, _ = conn.r.ReadBulkString()

	conn.log.Debug("SET %v, confirmed.", rsp.Id)
	conn.instance.SetResponse(rsp)
	if err := collector.Collect(collector.LogProxy, rsp.Cmd, rsp.Id.ReqId, rsp.Id.ChunkId, start.UnixNano(), int64(time.Since(start)), int64(0)); err != nil {
		conn.log.Warn("LogProxy err %v", err)
	}
}

func (conn *Connection) receiveData() {
	conn.log.Debug("DATA from lambda.")

	_, err := conn.r.ReadBulkString()
	ok, err := conn.r.ReadBulkString()
	if err != nil && err != io.EOF {
		conn.log.Error("Error on processing result of data collection: %v", err)
	}
	conn.log.Debug("Collected DATA from lambda: %s", ok)
	global.DataCollected.Done()
}

func (conn *Connection) initMigrateHandler() {
	conn.log.Debug("Request to initiate migration")

	conn.instance.Migrate()
}
