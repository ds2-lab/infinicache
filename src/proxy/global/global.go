package global

import (
	"fmt"
	"github.com/ScottMansfield/nanolog"
	"github.com/wangaoone/LambdaObjectstore/lib/logger"
	"github.com/wangaoone/redeo"
	"github.com/wangaoone/redeo/resp"
	"sync"
	"time"
)

var (
	Clients        = make([]chan interface{}, 1024*1024)
	Stores         *redeo.Group
	DataCollected  sync.WaitGroup
	LastActivity   = time.Now()
	Log            logger.ILogger

	reqMap         = make(map[string]*dataEntry)
	logMu          sync.Mutex
)

type dataEntry struct {
	cmd           string
	reqId         string
	chunkId       int64
	start         int64
	duration      int64
	firstByte     int64
	lambda2Server int64
	server2Client int64
	readBulk      int64
	appendBulk    int64
	flush         int64
}

func NanoLog(handle nanolog.Handle, args ...interface{}) error {
	LastActivity = time.Now()
	if handle == resp.LogStart {
		key := fmt.Sprintf("%s-%s-%d", args[0], args[1], args[2])
		logMu.Lock()
		reqMap[key] = &dataEntry{
			cmd:     args[0].(string),
			reqId:   args[1].(string),
			chunkId: args[2].(int64),
			start:   args[3].(int64),
		}
		logMu.Unlock()
		return nil
	} else if handle == resp.LogProxy {
		key := fmt.Sprintf("%s-%s-%d", args[0], args[1], args[2])
		logMu.Lock()
		entry := reqMap[key]
		logMu.Unlock()

		entry.firstByte = args[3].(int64) - entry.start
		args[3] = entry.firstByte
		entry.lambda2Server = args[4].(int64)
		entry.readBulk = args[5].(int64)
		return nil
	} else if handle == resp.LogServer2Client {
		key := fmt.Sprintf("%s-%s-%d", args[0], args[1], args[2])
		logMu.Lock()
		entry := reqMap[key]
		//delete(reqMap, key)
		logMu.Unlock()

		entry.server2Client = args[3].(int64)
		entry.appendBulk = args[4].(int64)
		entry.flush = args[5].(int64)
		entry.duration = args[6].(int64) - entry.start

		return nanolog.Log(resp.LogData, entry.cmd, entry.reqId, entry.chunkId,
			entry.start, entry.duration,
			entry.firstByte, entry.lambda2Server, entry.server2Client,
			entry.readBulk, entry.appendBulk, entry.flush)
	}

	return nanolog.Log(handle, args...)
}
