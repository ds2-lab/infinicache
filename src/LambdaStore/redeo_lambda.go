package main

import (
//	"bytes"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
  "github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/wangaoone/LambdaObjectstore/lib/logger"
	"github.com/wangaoone/redeo"
	"github.com/wangaoone/redeo/resp"
//	"github.com/wangaoone/s3gof3r"
//	"io"
	"net"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	prototol "github.com/wangaoone/LambdaObjectstore/src/types"
	"github.com/wangaoone/LambdaObjectstore/src/LambdaStore/types"
	lambdaTimeout "github.com/wangaoone/LambdaObjectstore/src/LambdaStore/timeout"
)

const OP_GET = "1"
const OP_SET = "0"
const EXPECTED_GOMAXPROCS = 3

var (
	//server     = "184.73.144.223:6379" // 10Gbps ec2 server UbuntuProxy0
	//server = "172.31.84.57:6379" // t2.micro ec2 server UbuntuProxy0 private ip under vpc
	server     = "52.87.169.1:6379" // t2.micro ec2 server UbuntuProxy0 public ip under vpc
	lambdaConn net.Conn
	srv        = redeo.NewServer(nil)
	myMap      = make(map[string]*types.Chunk)
	isFirst    = true
	log        = &logger.ColorLogger{
		Level: logger.LOG_LEVEL_WARN,
	}
	dataGatherer = make(chan *types.DataEntry, 10)
	dataDepository = make([]*types.DataEntry, 0, 100)
	dataDeposited sync.WaitGroup
	timeout = lambdaTimeout.New(0)

	active  int32
	done    chan struct{}
	id      uint64
)

func init() {
	goroutings := runtime.GOMAXPROCS(0)
	if goroutings < EXPECTED_GOMAXPROCS {
		log.Debug("Set GOMAXPROCS to %d (original %d)", EXPECTED_GOMAXPROCS, goroutings)
		runtime.GOMAXPROCS(EXPECTED_GOMAXPROCS)
	} else {
		log.Debug("GOMAXPROCS %d", goroutings)
	}
	timeout.SetLogger(log)
	adapt()
}

func adapt() {
	if lambdacontext.MemoryLimitInMB < 896 {
		lambdaTimeout.TICK_ERROR_EXTEND = lambdaTimeout.TICK_1_ERROR_EXTEND
		lambdaTimeout.TICK_ERROR = lambdaTimeout.TICK_1_ERROR
	} else if lambdacontext.MemoryLimitInMB < 1792 {
		lambdaTimeout.TICK_ERROR_EXTEND = lambdaTimeout.TICK_5_ERROR_EXTEND
		lambdaTimeout.TICK_ERROR = lambdaTimeout.TICK_5_ERROR
	} else {
		lambdaTimeout.TICK_ERROR_EXTEND = lambdaTimeout.TICK_10_ERROR_EXTEND
		lambdaTimeout.TICK_ERROR = lambdaTimeout.TICK_10_ERROR
	}
}

func HandleRequest(input prototol.InputEvent) error {
	timeout.Restart()
	atomic.StoreInt32(&active, 0)
	done = make(chan struct{})
	var clear sync.WaitGroup

	id = input.Id

	if isFirst == true {
		timeout.ResetWithExtension(lambdaTimeout.TICK_ERROR)

		log.Debug("Ready to connect %s, id %d", server, id)
		var connErr error
		lambdaConn, connErr = net.Dial("tcp", server)
		if connErr != nil {
			log.Error("Failed to connect server %s: %v", server, connErr)
			return connErr
		}
		log.Info("Connection to %v established (%v)", lambdaConn.RemoteAddr(), timeout.Since())

		isFirst = false
		go func() {
			srv.Serve_client(lambdaConn)
		}()
	} else {
		timeout.ResetWithExtension(lambdaTimeout.TICK_ERROR_EXTEND)
	}
	// append PONG back to proxy on being triggered
	pongHandler(lambdaConn)

	// data gathering
	go func(clear *sync.WaitGroup) {
		clear.Add(1)
		defer clear.Done()

		for {
			select {
			case <-done:
				return
			case entry := <-dataGatherer:
				dataDepository = append(dataDepository, entry)
				dataDeposited.Done()
			}
		}
	}(&clear)

	// timeout control
	func() {
		for {
			select {
			case <-done:
				return
			case <-timeout.C:
				if atomic.LoadInt32(&active) > 0 {
					timeout.Reset()
					break
				}
				log.Debug("Lambda timeout, return(%v).", timeout.Since())
				Done()
				return
			}
		}
	}()

	clear.Wait()
	log.Debug("All routing cleared at %v", timeout.Since())
	done = nil
	return nil
}

func Done() {
	select {
	case <-done:
		// closed
	default:
		close(done)
	}
}

func pongHandler(conn net.Conn) {
	pongWriter := resp.NewResponseWriter(conn)
	pong(pongWriter)
}

func pong(w resp.ResponseWriter) {
	w.AppendBulkString("pong")
	w.AppendInt(int64(id))
	if err := w.Flush(); err != nil {
		log.Error("Error on PONG flush: %v", err)
		return
	}
	log.Debug("PONG(%v)", timeout.Since())
}

// func remoteGet(bucket string, key string) []byte {
// 	log.Debug("get from remote storage")
// 	k, err := s3gof3r.EnvKeys()
// 	if err != nil {
// 		log.Debug("EnvKeys error: %v", err)
// 	}
//
// 	s3 := s3gof3r.New("", k)
// 	b := s3.Bucket(bucket)
//
// 	reader, _, err := b.GetReader(key, nil)
// 	if err != nil {
// 		log.Debug("GetReader error: %v", err)
// 	}
// 	obj := streamToByte(reader)
// 	return obj
// }
//
// func streamToByte(stream io.Reader) []byte {
// 	buf := new(bytes.Buffer)
// 	_, err := buf.ReadFrom(stream)
// 	if err != nil {
// 		log.Debug("ReadFrom error: %v", err)
// 	}
// 	return buf.Bytes()
// }

func main() {
	// Define handlers
	srv.HandleFunc("get", func(w resp.ResponseWriter, c *resp.Command) {
		atomic.AddInt32(&active, 1)
		defer atomic.AddInt32(&active, -1)
		defer timeout.ResetWithExtension(lambdaTimeout.TICK_ERROR)

		t := time.Now()
		log.Debug("In GET handler")

		connId := c.Arg(0).String()
		reqId := c.Arg(1).String()
		key := c.Arg(3).String()

		//val, err := myCache.Get(key)
		//if err == false {
		//	log.Debug("not found")
		//}
		chunk, found := myMap[key]
		if found == false {
			log.Debug("%s not found", key)
			dataDeposited.Add(1)
			dataGatherer <- &types.DataEntry{OP_GET, "404", reqId, "-1", 0, 0, time.Since(t)}
			return
		}

		// construct lambda store response
		w.AppendBulkString("get")
		w.AppendBulkString(connId)
		w.AppendBulkString(reqId)
		w.AppendBulkString(chunk.Id)
		t2 := time.Now()
		w.AppendBulk(chunk.Body)
		d2 := time.Since(t2)

		t3 := time.Now()
		if err := w.Flush(); err != nil {
			log.Error("Error on get::flush(key %s): %v", key, err)
			dataDeposited.Add(1)
			dataGatherer <- &types.DataEntry{OP_GET, "500", reqId, chunk.Id, d2, 0, time.Since(t)}
			return
		}
		d3 := time.Since(t3)
		dt := time.Since(t)

		log.Debug("AppendBody duration is %v", d2)
		log.Debug("Flush duration is %v", d3)
		log.Debug("Total duration is %v", dt)
		log.Debug("Get complete, Key: %s, ConnID:%s, ChunkID:%s", key, connId, chunk.Id)
		dataDeposited.Add(1)
		dataGatherer <- &types.DataEntry{OP_GET, "200", reqId, chunk.Id, d2, d3, dt}
	})

	srv.HandleFunc("set", func(w resp.ResponseWriter, c *resp.Command) {
		atomic.AddInt32(&active, 1)
		defer atomic.AddInt32(&active, -1)
		defer timeout.ResetWithExtension(lambdaTimeout.TICK_ERROR)

		t := time.Now()
		log.Debug("In SET handler")
		//if c.ArgN() != 3 {
		//	w.AppendError(redeo.WrongNumberOfArgs(c.Name))
		//	return
		//}

		connId := c.Arg(0).String()
		reqId := c.Arg(1).String()
		chunkId := c.Arg(2).String()
		key := c.Arg(3).String()
		val := c.Arg(4).Bytes()
		myMap[key] = &types.Chunk{chunkId, val}

		// write Key, clientId, chunkId, body back to server
		w.AppendBulkString("set")
		w.AppendBulkString(connId)
		w.AppendBulkString(reqId)
		w.AppendBulkString(chunkId)
		if err := w.Flush(); err != nil {
			log.Error("Error on set::flush(key %s): %v", key, err)
			dataDeposited.Add(1)
			dataGatherer <- &types.DataEntry{OP_SET, "500", reqId, chunkId, 0, 0, time.Since(t)}
			return
		}

		log.Debug("Set complete, Key:%s, ConnID: %s, ChunkID: %s, Item length %d", key, connId, chunkId, len(val))
		dataDeposited.Add(1)
		dataGatherer <- &types.DataEntry{OP_SET, "200", reqId, chunkId, 0, 0, time.Since(t)}
	})

	srv.HandleFunc("data", func(w resp.ResponseWriter, c *resp.Command) {
		timeout.Stop()

		log.Debug("In DATA handler")

		// Wait for data depository.
		dataDeposited.Wait()

		w.AppendBulkString("data")
		w.AppendBulkString(strconv.Itoa(len(dataDepository)))
		for _, entry := range dataDepository {
			format := fmt.Sprintf("%s,%s,%s,%s,%d,%d,%d",
				entry.Op, entry.ReqId, entry.ChunkId, entry.Status,
				entry.Duration, entry.DurationAppend, entry.DurationFlush)
			w.AppendBulkString(format)

			//w.AppendBulkString(entry.Op)
			//w.AppendBulkString(entry.Status)
			//w.AppendBulkString(entry.ReqId)
			//w.AppendBulkString(entry.ChunkId)
			//w.AppendBulkString(entry.DurationAppend.String())
			//w.AppendBulkString(entry.DurationFlush.String())
			//w.AppendBulkString(entry.Duration.String())
		}
		if err := w.Flush(); err != nil {
			log.Error("Error on data::flush: %v", err)
			return
		}
		log.Debug("data complete")
		lambdaConn.Close()
		lambdaConn = nil
		// No need to close server, it will serve the new connection next time.
		dataDepository = dataDepository[:0]
		isFirst = true
		Done()
	})

	srv.HandleFunc("ping", func(w resp.ResponseWriter, c *resp.Command) {
		atomic.AddInt32(&active, 1)
		defer atomic.AddInt32(&active, -1)
		defer timeout.ResetWithExtension(lambdaTimeout.TICK_ERROR_EXTEND)

		log.Debug("PING")
		pong(w)
	})

	lambda.Start(HandleRequest)
}
