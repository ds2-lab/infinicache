package lambdastore

import (
	"errors"
	"fmt"
	"github.com/wangaoone/LambdaObjectstore/lib/logger"
	"github.com/wangaoone/redeo"
	"github.com/wangaoone/redeo/resp"
	"io"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/wangaoone/LambdaObjectstore/src/proxy/global"
)

type Connection struct {
	instance     *Instance
	log          logger.ILogger
	cn           net.Conn
	w            *resp.RequestWriter
	r            resp.ResponseReader
	closed       chan struct{}
}

func NewConnection(c net.Conn) *Connection {
	return &Connection{
		log: &logger.ColorLogger{
			Prefix: fmt.Sprintf("Undesignated "),
			Level: global.Log.GetLevel(),
			Color: true,
		},
		cn: c,
		// wrap writer and reader
		w: resp.NewRequestWriter(c),
		r: resp.NewResponseReader(c),
		closed: make(chan struct{}),
	}
}

func (conn *Connection) Close() {
	select {
	case <-conn.closed:
		// already closed
		return
	default:
	}

	if conn.cn != nil {
		// Don't use c.Close(), it will stuck and wait for lambda.
		conn.cn.(*net.TCPConn).SetLinger(0) // The operating system discards any unsent or unacknowledged data.
		conn.cn.Close()
	}
	close(conn.closed)
}

func (conn *Connection) IsClosed() bool {
	select {
	case <-conn.closed:
		return true
	default:
		return false
	}
}

// Handle incoming client requests
func (conn *Connection) handleRequests() {
	var isDataRequest bool
	for {
		select {
		case <-conn.closed:
			return
		case req := <-conn.instance.C(): /*blocking on lambda facing channel*/
			// check lambda status first
			conn.instance.Validate()

			// get arguments
			connId := strconv.Itoa(req.Id.ConnId)
			chunkId := strconv.FormatInt(req.Id.ChunkId, 10)

			cmd := strings.ToLower(req.Cmd)
			isDataRequest = false
			switch cmd {
			case "set": /*set or two argument cmd*/
				conn.w.MyWriteCmd(req.Cmd, connId, req.Id.ReqId, chunkId, req.Key, req.Body)
			case "get": /*get or one argument cmd*/
				conn.w.MyWriteCmd(req.Cmd, connId, req.Id.ReqId, "", req.Key)
			case "data":
				conn.w.WriteCmdString(req.Cmd)
				isDataRequest = true
			}
			err := conn.w.Flush()
			if err != nil {
				conn.log.Error("Flush pipeline error: %v", err)
				if isDataRequest {
					global.DataCollected.Done()
				}
			}
		}
	}
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
		// field 0 for cmd
		field0, err := conn.r.PeekType()
		if err != nil {
			if err == io.EOF {
				conn.log.Warn("Lambda store disconnected.")
				conn.cn = nil
				conn.Close()
				return
			} else {
				conn.log.Warn("Failed to peek response type: %v", err)
			}
			continue
		}
		start := time.Now()

		var cmd string
		switch field0 {
		case resp.TypeError:
			strErr, _ := conn.r.ReadError()
			err = errors.New(strErr)
			conn.log.Warn("Error on peek response type: %v", err)
		default:
			cmd, err = conn.r.ReadBulkString()
			if err != nil {
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
			default:
				conn.log.Warn("Unsupport response type: %s", cmd)
			}
		}
	}
}

func (conn *Connection) pongHandler() {
	conn.log.Debug("PONG from lambda.")

	// Read lambdaId
	id, _ := conn.r.ReadInt()

	// Lock up lambda instance
	instance := global.Stores.All[int(id)].(*Instance)
	if instance.cn != conn {
		// Set instance, order matters here.
		conn.instance = instance
		conn.log = instance.log
		conn.log.Debug("PONG from lambda confirmed.")
		if conn.instance.cn != nil && !conn.instance.cn.IsClosed() {
			conn.instance.cn.Close()
		}
		conn.instance.cn = conn

		// start handle requests from client.
		go conn.handleRequests()
	}

	select {
	case <-conn.instance.validated:
		// Validated
	default:
		conn.log.Info("Validated")
		close(conn.instance.validated)
	}
}

func (conn *Connection) getHandler(start time.Time) {
	conn.log.Debug("GET from lambda.")

	// Exhaust all values to keep protocol aligned.
	connId, _ := conn.r.ReadBulkString()
	reqId, _ := conn.r.ReadBulkString()
	chunkId, _ := conn.r.ReadBulkString()
	counter, ok := redeo.ReqMap.Get(reqId)
	if ok == false {
		conn.log.Warn("Request not found: %s", reqId)
		// exhaust value field
		conn.r.ReadBulk(nil)
		return
	}

	var obj redeo.Response
	obj.Cmd = "get"
	obj.Id.ConnId, _ = strconv.Atoi(connId)
	obj.Id.ReqId = reqId
	obj.Id.ChunkId, _ = strconv.ParseInt(chunkId, 10, 64)

	abandon := false
	reqCounter := atomic.AddInt32(&(counter.(*redeo.ClientReqCounter).Counter), 1)
	// Check if chunks are enough? Shortcut response if YES.
	if int(reqCounter) > counter.(*redeo.ClientReqCounter).DataShards {
		abandon = true
		global.Clients[obj.Id.ConnId] <- &redeo.Chunk{ChunkId: obj.Id.ChunkId, ReqId: obj.Id.ReqId, Cmd: obj.Cmd}
		if err := global.NanoLog(resp.LogProxy, obj.Cmd, obj.Id.ReqId, obj.Id.ChunkId, start.UnixNano(), int64(time.Since(start)), int64(0)); err != nil {
			conn.log.Warn("LogProxy err %v", err)
		}
	}

	// Read value
	t9 := time.Now()
	res, err := conn.r.ReadBulk(nil)
	time9 := time.Since(t9)
	if err != nil {
		conn.log.Warn("Failed to read value of response: %v", err)
		// Abandon errant data
		res = nil
	}
	// Skip on abandon
	if abandon {
		return
	}

	conn.log.Debug("GET peek complete, send to client channel", connId, obj.Id.ReqId, chunkId)
	global.Clients[obj.Id.ConnId] <- &redeo.Chunk{ChunkId: obj.Id.ChunkId, ReqId: obj.Id.ReqId, Body: res, Cmd: obj.Cmd}
	if err := global.NanoLog(resp.LogProxy, obj.Cmd, obj.Id.ReqId, obj.Id.ChunkId, start.UnixNano(), int64(time.Since(start)), int64(time9)); err != nil {
		conn.log.Warn("LogProxy err %v", err)
	}
}

func (conn *Connection) setHandler(start time.Time) {
	conn.log.Debug("SET from lambda.")

	var obj redeo.Response
	connId, _ := conn.r.ReadBulkString()
	obj.Id.ConnId, _ = strconv.Atoi(connId)
	obj.Id.ReqId, _ = conn.r.ReadBulkString()
	chunkId, _ := conn.r.ReadBulkString()
	obj.Id.ChunkId, _ = strconv.ParseInt(chunkId, 10, 64)

	conn.log.Debug("SET peek complete, send to client channel, %s,%s,%s", connId, obj.Id.ReqId, chunkId)
	global.Clients[obj.Id.ConnId] <- &redeo.Chunk{ChunkId: obj.Id.ChunkId, ReqId: obj.Id.ReqId, Body: []byte{1}, Cmd: "set"}
	if err := global.NanoLog(resp.LogProxy, "set", obj.Id.ReqId, obj.Id.ChunkId, start.UnixNano(), int64(time.Since(start)), int64(0)); err != nil {
		conn.log.Warn("LogProxy err %v", err)
	}
}

func (conn *Connection) receiveData() {
	conn.log.Debug("DATA from lambda.")

	strLen, err := conn.r.ReadBulkString()
	len := 0
	if err != nil {
		conn.log.Error("Failed to read length of data from lambda: %v", err)
	} else {
		len, err = strconv.Atoi(strLen)
		if err != nil {
			conn.log.Error("Convert strLen err: %v", err)
		}
	}
	for i := 0; i < len; i++ {
		//op, _ := conn.r.ReadBulkString()
		//status, _ := conn.r.ReadBulkString()
		//reqId, _ := conn.r.ReadBulkString()
		//chunkId, _ := conn.r.ReadBulkString()
		//dAppend, _ := conn.r.ReadBulkString()
		//dFlush, _ := conn.r.ReadBulkString()
		//dTotal, _ := conn.r.ReadBulkString()
		dat, _ := conn.r.ReadBulkString()
		//fmt.Println("op, reqId, chunkId, status, dTotal, dAppend, dFlush", op, reqId, chunkId, status, dTotal, dAppend, dFlush)
		global.NanoLog(resp.LogLambda, "data", dat)
	}
	conn.log.Info("Data collected, %d in total.", len)
	global.DataCollected.Done()
}
