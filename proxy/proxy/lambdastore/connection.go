package lambdastore

import (
	"errors"
	"fmt"
	"github.com/neboduus/infinicache/proxy/common/logger"
	"github.com/mason-leap-lab/redeo/resp"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/neboduus/infinicache/proxy/proxy/collector"
	"github.com/neboduus/infinicache/proxy/proxy/global"
	"github.com/neboduus/infinicache/proxy/proxy/types"
)

var (
	defaultConnectionLog = &logger.ColorLogger{
		Prefix: fmt.Sprintf("Undesignated "),
		Color:  true,
	}
	ErrConnectionClosed = errors.New("Connection closed")
	ErrMissingResponse  = errors.New("Missing response")
)

type Connection struct {
	instance *Instance
	log      logger.ILogger
	cn       net.Conn
	w        *resp.RequestWriter
	r        resp.ResponseReader
	mu       sync.Mutex
	chanWait chan *types.Request
	respType chan interface{}
	closed   chan struct{}
}

func NewConnection(c net.Conn) *Connection {
	conn := &Connection{
		log: defaultConnectionLog,
		cn:  c,
		// wrap writer and reader
		w:        resp.NewRequestWriter(c),
		r:        resp.NewResponseReader(c),
		chanWait: make(chan *types.Request, 1),
		respType: make(chan interface{}),
		closed:   make(chan struct{}),
	}
	defaultConnectionLog.Level = global.Log.GetLevel()
	return conn
}

func (conn *Connection) Close() {
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
	conn.log.Debug("Signal to close.")
}

func (conn *Connection) close() {
	conn.Close()
	conn.bye()
	// Don't use c.Close(), it will stuck and wait for lambda.
	conn.cn.(*net.TCPConn).SetLinger(0) // The operating system discards any unsent or unacknowledged data.
	conn.cn.Close()
	conn.clearResponses()
	conn.log.Debug("Closed.")
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
			if len(conn.chanWait) > 0 {
				// Is there request waiting?
				retPeek = <-conn.respType
			} else {
				conn.close()
				return
			}
		case retPeek = <-conn.respType:
			// Got response, reset read deadline.
			conn.cn.SetReadDeadline(time.Time{})
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
			conn.close()
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
			conn.SetErrorResponse(err)
		default:
			cmd, err := conn.r.ReadBulkString()
			if err != nil && err == io.EOF {
				conn.log.Warn("Lambda store disconnected.")
				conn.close()
			} else if err != nil {
				conn.log.Warn("Error on read response type: %v", err)
				break
			}

			switch cmd {
			case "pong":
				conn.pongHandler()
			case "get":
				conn.getHandler(start)
			case "mkget":
				conn.mkGetHandler(start)
			case "set":
				conn.setHandler(start)
			case "mkset":
				conn.mkSetHandler(start)
			case "del":
				conn.delHandler()
			case "data":
				conn.receiveData()
			case "initMigrate":
				go conn.initMigrateHandler()
			case "bye":
				conn.bye()
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

func (conn *Connection) SetResponse(rsp *types.Response) (*types.Request, bool) {
	if len(conn.chanWait) == 0 {
		conn.log.Error("Unexpected response: %v", rsp)
		return nil, false
	}
	for req := range conn.chanWait {
		if req.IsResponse(rsp) {
			conn.log.Debug("response matched: %v", req.Id)

			return req, req.SetResponse(rsp)
		}
		conn.log.Warn("passing req: %v, got %v", req, rsp)
		req.SetResponse(ErrMissingResponse)

		if len(conn.chanWait) == 0 {
			break
		}
	}
	return nil, false
}

func (conn *Connection) SetErrorResponse(err error) {
	if len(conn.chanWait) == 0 {
		conn.log.Error("Unexpected error response: %v", err)
		return
	}
	for req := range conn.chanWait {
		req.SetResponse(err)
		break
	}
}

func (conn *Connection) clearResponses() {
	if len(conn.chanWait) == 0 {
		return
	}
	for req := range conn.chanWait {
		req.SetResponse(ErrConnectionClosed)
		if len(conn.chanWait) == 0 {
			break
		}
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
		req, ok := conn.SetResponse(rsp)
		if ok && req.EnableCollector {
			err := collector.Collect(collector.LogProxy, rsp.Cmd, rsp.Id.ReqId, rsp.Id.ChunkId, start.UnixNano(), int64(time.Since(start)), int64(0))
			if err != nil {
				conn.log.Warn("LogProxy err %v", err)
			}
		}
		if int(reqCounter) == counter.(*types.ClientReqCounter).DataShards+counter.(*types.ClientReqCounter).ParityShards {
			global.ReqMap.Del(reqId)
		}
	}

	// Read value
	// t9 := time.Now()
	// bodyStream, err := conn.r.ReadBulk(nil)
	// time9 := time.Since(t9)
	// if err != nil {
	// 	conn.log.Warn("Failed to read value of response: %v", err)
	// 	// Abandon errant data
	// 	res.csv = nil
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
	if req, ok := conn.SetResponse(rsp); !ok {
		// Failed to set response, release hold.
		rsp.BodyStream.(resp.Holdable).Unhold()
	} else if req.EnableCollector {
		err := collector.Collect(collector.LogProxy, rsp.Cmd, rsp.Id.ReqId, rsp.Id.ChunkId, start.UnixNano(), int64(time.Since(start)), int64(0))
		if err != nil {
			conn.log.Warn("LogProxy err %v", err)
		}
	}
}

func (conn *Connection) setHandler(start time.Time) {
	conn.log.Debug("SET from lambda.")

	rsp := &types.Response{Cmd: "set", Body: []byte(strconv.FormatUint(conn.instance.Id(), 10))}
	connId, _ := conn.r.ReadBulkString()
	rsp.Id.ConnId, _ = strconv.Atoi(connId)
	rsp.Id.ReqId, _ = conn.r.ReadBulkString()
	rsp.Id.ChunkId, _ = conn.r.ReadBulkString()

	conn.log.Debug("SET %v, confirmed.", rsp.Id)
	req, ok := conn.SetResponse(rsp)
	if ok && req.EnableCollector {
		err := collector.Collect(collector.LogProxy, rsp.Cmd, rsp.Id.ReqId, rsp.Id.ChunkId, start.UnixNano(), int64(time.Since(start)), int64(0))
		if err != nil {
			conn.log.Warn("LogProxy err %v", err)
		}
	}
}

func (conn *Connection) mkGetHandler(start time.Time)  {
	conn.log.Debug("mkGET from lambda %d", conn.instance.id)

	// Exhaust all values to keep protocol aligned.
	connId, _ := conn.r.ReadBulkString()
	reqId, _ := conn.r.ReadBulkString()
	chunkId, _ := conn.r.ReadBulkString()
	lowLevelKeysN, _ := conn.r.ReadInt()

	lowLevelKeyPairs := make(map[string][]byte)
	for i:=0;i<int(lowLevelKeysN);i++{
		lowLevelKey, _ := conn.r.ReadBulkString()
		var lowLevelValue []byte
		lowLevelValue, _ = conn.r.ReadBulk(lowLevelValue)
		lowLevelKeyPairs[lowLevelKey] = lowLevelValue
	}
	//conn.log.Debug("received: %v", lowLevelKeyPairs)
	_, ok := global.ReqMap.Get(reqId) //req counter
	if ok == false {
		conn.log.Warn("Request not found: %s", reqId)
		// exhaust value field
		if err := conn.r.SkipBulk(); err != nil {
			conn.log.Warn("Failed to skip bulk on request mismatch: %v", err)
		}
		return
	}

	rsp := &types.Response{Cmd: "mkget"}
	rsp.Id.ConnId, _ = strconv.Atoi(connId)
	rsp.Id.ReqId = reqId
	rsp.Id.ChunkId = chunkId
	rsp.LowLevelKeyValuePairs = lowLevelKeyPairs

	conn.log.Debug("mkGOT %v, confirmed.", rsp.Id)
	if req, ok := conn.SetResponse(rsp); !ok {
		// Failed to set response
		conn.log.Warn("LogProxy err in conn.mkGetHandler at conn.SetResponse",)
	} else if req.EnableCollector {
		err := collector.Collect(collector.LogProxy, rsp.Cmd, rsp.Id.ReqId, rsp.Id.ChunkId, start.UnixNano(), int64(time.Since(start)), int64(0))
		if err != nil {
			conn.log.Warn("LogProxy err %v", err)
		}
	}
}

func (conn *Connection) mkSetHandler(start time.Time) {
	conn.log.Debug("mkSET from lambda.")

	rsp := &types.Response{Cmd: "mkset", Body: []byte(strconv.FormatUint(conn.instance.Id(), 10))}
	connId, _ := conn.r.ReadBulkString()
	rsp.Id.ConnId, _ = strconv.Atoi(connId)
	rsp.Id.ReqId, _ = conn.r.ReadBulkString()
	rsp.Id.ChunkId, _ = conn.r.ReadBulkString()

	conn.log.Debug("mkSET %v, confirmed.", rsp.Id)
	req, ok := conn.SetResponse(rsp)
	if ok && req.EnableCollector {
		err := collector.Collect(collector.LogProxy, rsp.Cmd, rsp.Id.ReqId, rsp.Id.ChunkId, start.UnixNano(), int64(time.Since(start)), int64(0))
		if err != nil {
			conn.log.Warn("LogProxy err %v", err)
		}
	}
}

func (conn *Connection) delHandler() {
	conn.log.Debug("DEL from lambda.")

	rsp := &types.Response{Cmd: "del"}

	connId, _ := conn.r.ReadBulkString()
	rsp.Id.ConnId, _ = strconv.Atoi(connId)
	rsp.Id.ReqId, _ = conn.r.ReadBulkString()
	rsp.Id.ChunkId, _ = conn.r.ReadBulkString()

	conn.log.Debug("DEL %v, confirmed.", rsp.Id)
	conn.SetResponse(rsp) // if del is control cmd, should return False
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

func (conn *Connection) bye() {
	if conn.instance != nil {
		conn.instance.bye(conn)
	}
}
