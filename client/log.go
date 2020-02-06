package ecRedis

import (
	"fmt"
	"github.com/ScottMansfield/nanolog"
	"os"
)

var (
	//LogClient nanolog.Handle
	LogRec    nanolog.Handle
	LogDec    nanolog.Handle
	LogClient nanolog.Handle
	nlogger    func(nanolog.Handle, ...interface{}) error
)

func init() {
	//LogClient = nanolog.AddLogger("%s All goroutine has finished. Duration is %s")
	LogRec = nanolog.AddLogger("chunk id is %i, " +
		"Client send RECEIVE req timeStamp is %s " +
		"Client Peek ChunkId time is %s" +
		"Client read ChunkId time is %s " +
		"Client Peek chunkBody time is %s " +
		"Client read chunkBody time is %s " +
		"RECEIVE goroutine duration time is %s ")
	LogDec = nanolog.AddLogger("DataStatus is %b, Decoding time is %s")
	// cmd, reqId, Begin, duration, get/set req latency, rec latency, decoding latency
	LogClient = nanolog.AddLogger("%s,%s,%i64,%i64,%i64,%i64,%i64,%b,%b")

}

func CreateLog(opts map[string]interface{}) {
	path := opts["file"].(string) + "_bench.clog"
	nanoLogout, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	err = nanolog.SetWriter(nanoLogout)
	if err != nil {
		panic(err)
	}
}

func FlushLog() {
	if err := nanolog.Flush(); err != nil {
		fmt.Println("log flush err")
	}
}

func SetLogger(l func(nanolog.Handle, ...interface{}) error) {
	nlogger = l
}

func nanoLog(handle nanolog.Handle, args ...interface{}) {
	//if logger != nil {
	fmt.Println("nanoLog argu 0 is", args[0])
	nlogger(handle, args...)
	//}
}
