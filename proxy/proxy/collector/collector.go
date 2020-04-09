package collector

import (
	"errors"
	"fmt"
	"github.com/ScottMansfield/nanolog"
	"os"
	"sync"
	"time"

	"github.com/neboduus/infinicache/proxy/proxy/global"
)

var (
	LastActivity     = time.Now()
	LogServer2Client nanolog.Handle
	//LogServer        nanolog.Handle
	//LogServerBufio   nanolog.Handle
	LogProxy    nanolog.Handle = 10001
	LogData     nanolog.Handle = 10002
	LogStart    nanolog.Handle = 10003
	LogLambda   nanolog.Handle = 10004
	LogValidate nanolog.Handle = 10005
	ErrorNoEntry     = errors.New("No collector log entry found.")

	logMu   sync.Mutex
	ticker  *time.Ticker
	stopped bool
	reqMap  = make(map[string]*dataEntry)
)

func init() {
	//LogServer2Client = nanolog.AddLogger("Cmd is %s, ReqId is %s, ChunkId is %s, Server2Client total time is %i64, AppendBulk time is %i64, Flush time is %i64, TimeStamp is %i64")
	//LogServer = nanolog.AddLogger("KEY is %s, IN %s, ReqID is %s, ConnID is %i, ChunkID is %s, LambdaStoreID is %i64")
	//LogServerBufio = nanolog.AddLogger("ReadBulk ReadLen time is %s, ReadBulk Require time is %s, ReadBulk Append time is %s")
	//LogProxy = nanolog.AddLogger("Cmd is %s, ReqId is %s, ChunkId is %s, First byte is %i64, lambda2Server total time is %i64, Server read chunkBody is %i64")
	//"Sever read field0 clientId time is %s, " +
	//"Sever PeekType chunkId time is %s, " +
	//"Sever read field1 chunkId time is %s, " +
	//	"Sever PeekType objBody time is %s, " +
	LogData = nanolog.AddLogger("%s,%s,%s,%i64,%i64,%i64,%i64,%i64,%i64,%i64,%i64,%i64")
	//LogLambda = nanolog.AddLogger("%s,%s,%s,%s,%s,%s,%s,%s")
	LogLambda = nanolog.AddLogger("%s,%s")
}

func Create(prefix string) {
	// get local time
	//location, _ := time.LoadLocation("EST")
	// Set up nanoLog writer
	//nanoLogout, err := os.Create("/tmp/proxy/" + *prefix + "_proxy.clog")
	nanoLogout, err := os.Create(prefix + "_proxy.clog")
	if err != nil {
		panic(err)
	}
	err = nanolog.SetWriter(nanoLogout)
	if err != nil {
		panic(err)
	}

	go func() {
		ticker = time.NewTicker(1 * time.Second)
		for {
			<-ticker.C
			if stopped || time.Since(LastActivity) >= 10*time.Second {
				if err := nanolog.Flush(); err != nil {
					global.Log.Warn("Failed to save data: %v", err)
				}
			}
			if stopped {
				return
			}
		}
	}()
}

func Stop() {
	stopped = true
	if ticker != nil {
		ticker.Stop()
	}
}

func Flush() error {
	return nanolog.Flush()
}

type dataEntry struct {
	cmd           string
	reqId         string
	chunkId       string
	start         int64
	duration      int64
	firstByte     int64
	lambda2Server int64
	server2Client int64
	readBulk      int64
	appendBulk    int64
	flush         int64
	validate      int64
}

func Collect(handle nanolog.Handle, args ...interface{}) error {
	LastActivity = time.Now()
	if handle == LogStart {
		key := fmt.Sprintf("%s-%s-%s", args[0], args[1], args[2])
		logMu.Lock()
		reqMap[key] = &dataEntry{
			cmd:     args[0].(string),
			reqId:   args[1].(string),
			chunkId: args[2].(string),
			start:   args[3].(int64),
		}
		logMu.Unlock()
		return nil
	} else if handle == LogProxy {
		key := fmt.Sprintf("%s-%s-%s", args[0], args[1], args[2])
		logMu.Lock()
		entry, exist := reqMap[key]
		logMu.Unlock()

		if !exist {
			return ErrorNoEntry
		}
		entry.firstByte = args[3].(int64) - entry.start
		args[3] = entry.firstByte
		entry.lambda2Server = args[4].(int64)
		entry.readBulk = args[5].(int64)
		return nil
	} else if handle == LogServer2Client {
		key := fmt.Sprintf("%s-%s-%s", args[0], args[1], args[2])
		logMu.Lock()
		entry, exist := reqMap[key]
		//delete(reqMap, key)
		logMu.Unlock()

		if !exist {
			return ErrorNoEntry
		}
		entry.server2Client = args[3].(int64)
		entry.appendBulk = args[4].(int64)
		entry.flush = args[5].(int64)
		entry.duration = args[6].(int64) - entry.start

		return nanolog.Log(LogData, entry.cmd, entry.reqId, entry.chunkId,
			entry.start, entry.duration,
			entry.firstByte, entry.lambda2Server, entry.server2Client,
			entry.readBulk, entry.appendBulk, entry.flush, entry.validate)
	} else if handle == LogValidate {
		key := fmt.Sprintf("%s-%s-%s", args[0], args[1], args[2])
		logMu.Lock()
		entry, exist := reqMap[key]
		logMu.Unlock()

		if !exist {
			return ErrorNoEntry
		}
		entry.validate = args[3].(int64)
		return nil
	}

	return nanolog.Log(handle, args...)
}
