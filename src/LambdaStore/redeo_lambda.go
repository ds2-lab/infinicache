package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	lambdaService "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/wangaoone/LambdaObjectstore/lib/logger"
	"github.com/wangaoone/redeo"
	"github.com/wangaoone/redeo/resp"
	"os/exec"
	"strings"

	//	"github.com/wangaoone/s3gof3r"
	"io"
	"net"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	lambdaTimeout "github.com/wangaoone/LambdaObjectstore/src/LambdaStore/timeout"
	"github.com/wangaoone/LambdaObjectstore/src/LambdaStore/types"
	protocol "github.com/wangaoone/LambdaObjectstore/src/types"
)

const (
	OP_GET              = "1"
	OP_SET              = "0"
	EXPECTED_GOMAXPROCS = 2
	LIFESPAN            = 60
	STATUSCODE          = 202
)

var (
	startTime = time.Now()
	//server     = "184.73.144.223:6379" // 10Gbps ec2 server UbuntuProxy0
	//server = "172.31.84.57:6379" // t2.micro ec2 server UbuntuProxy0 private ip under vpc
	server     = "18.214.26.94:6379" // t2.micro ec2 server UbuntuProxy0 public ip under vpc
	lambdaConn net.Conn
	srv        = redeo.NewServer(nil)
	myMap      = make(map[string]*types.Chunk)
	isFirst    = true
	log        = &logger.ColorLogger{
		Level: logger.LOG_LEVEL_WARN,
	}
	dataGatherer   = make(chan *types.DataEntry, 10)
	dataDepository = make([]*types.DataEntry, 0, 100)
	dataDeposited  sync.WaitGroup
	timeout        = lambdaTimeout.New(0)
	// Pong limiter prevent pong being sent duplicatedly on launching lambda while a ping arrives
	// at the same time.
	pongLimiter = make(chan struct{}, 1)

	active       int32
	mu           sync.RWMutex
	done         chan struct{}
	id           uint64
	hostName     string
	lambdaReqId  string
	addrMigrator string
	srcConn      net.Conn
	dstConn      net.Conn
	migrateChan  = make(chan error, 1)
)

func init() {
	goroutines := runtime.GOMAXPROCS(0)
	if goroutines < EXPECTED_GOMAXPROCS {
		log.Debug("Set GOMAXPROCS to %d (original %d)", EXPECTED_GOMAXPROCS, goroutines)
		runtime.GOMAXPROCS(EXPECTED_GOMAXPROCS)
	} else {
		log.Debug("GOMAXPROCS %d", goroutines)
	}
	timeout.SetLogger(log)
	adapt()

	cmd := exec.Command("uname", "-a")
	host, err := cmd.CombinedOutput()
	if err != nil {
		log.Debug("cmd.Run() failed with %s\n", err)
	}

	hostName = strings.Split(string(host), " #")[0]
	log.Debug("hostname is: %s", hostName)
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

func getAwsReqId(ctx context.Context) string {
	lc, ok := lambdacontext.FromContext(ctx)
	if ok == false {
		log.Debug("get lambda context failed %v", ok)
	}
	return lc.AwsRequestID
}

func HandleRequest(ctx context.Context, input protocol.InputEvent) error {
	// get lambda invoke reqId
	lambdaReqId = getAwsReqId(ctx)

	// get duration time from first invoke
	invokeAge := time.Since(startTime).Minutes()
	if invokeAge >= LIFESPAN {
		err := initBackup(lambdaConn)
		if err != nil {
			log.Error("init backup err %v", err)
			return err
		}
		log.Debug("initBackup success")
	}

	// migration triggered lambda
	if input.Cmd != "migrate" {
		var connErr error
		// dial to migrator
		dstConn, connErr = net.Dial("tcp", input.MigratorAddr)
		if connErr != nil {
			log.Error("Failed to connect migrator %s: %v", input.MigratorAddr, connErr)
			return connErr
		}
		// dial to proxy
		lambdaConn, connErr = net.Dial("tcp", server)
		if connErr != nil {
			log.Error("Failed to connect server %s: %v", server, connErr)
			return connErr
		}
		log.Info("Connection to %v established (%v)", lambdaConn.RemoteAddr(), timeout.Since())

	}

	if input.Timeout > 0 {
		deadline, _ := ctx.Deadline()
		timeout.RestartWithCalibration(deadline.Add(-time.Duration(input.Timeout) * time.Second))
	} else {
		timeout.Restart()
	}
	atomic.StoreInt32(&active, 0)
	ResetDone()
	var clear sync.WaitGroup
	issuePong()

	// log.Debug("Routings on requesting: %d", runtime.NumGoroutine())

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
			err := srv.ServeForeignClient(lambdaConn)
			if err != nil && err != io.EOF {
				log.Info("Connection closed: %v", err)
			} else {
				log.Info("Connection closed.")
			}
			lambdaConn = nil
			isFirst = true
			Done()
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
				mu.Lock()

				if atomic.LoadInt32(&active) > 0 {
					timeout.Reset()
					mu.Unlock()
					break
				}
				doneLocked()
				mu.Unlock()
				log.Debug("Lambda timeout, return(%v).", timeout.Since())
				return
			}
		}
	}()

	clear.Wait()
	log.Debug("All routing cleared(%d) at %v", runtime.NumGoroutine(), timeout.Since())
	ClearDone()
	return nil
}

func ResetDone() {
	mu.Lock()
	defer mu.Unlock()

	resetDoneLocked()
}

func Done() {
	mu.Lock()
	defer mu.Unlock()

	resetDoneLocked()
	doneLocked()
}

func ClearDone() {
	mu.Lock()
	defer mu.Unlock()

	done = nil
}

func IsDone() bool {
	mu.RLock()
	defer mu.RUnlock()

	if done == nil {
		return true
	}

	select {
	case <-done:
		return true
	default:
		return false
	}
}

func resetDoneLocked() {
	if done == nil {
		done = make(chan struct{})
	}
}

func doneLocked() {
	select {
	case <-done:
		// closed
	default:
		close(done)
	}
}

func issuePong() {
	mu.RLock()
	defer mu.RUnlock()

	select {
	case pongLimiter <- struct{}{}:
	default:
		// if limiter is full, move on
	}
}

func pongHandler(conn net.Conn) {
	pongWriter := resp.NewResponseWriter(conn)
	pong(pongWriter)
}

func pong(w resp.ResponseWriter) {
	mu.RLock()
	defer mu.RUnlock()

	select {
	case <-pongLimiter:
		// Quota avaiable or abort.
	default:
		return
	}

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
		timeout.Requests++
		extension := lambdaTimeout.TICK_ERROR
		if timeout.Requests > 1 {
			extension = lambdaTimeout.TICK
		}
		defer func() {
			timeout.ResetWithExtension(extension)
			atomic.AddInt32(&active, -1)
		}()

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
			log.Warn("%s not found", key)
			w.AppendErrorf("%s not found", key)
			if err := w.Flush(); err != nil {
				log.Error("Error on flush(error 404): %v", err)
			}
			dataDeposited.Add(1)
			dataGatherer <- &types.DataEntry{OP_GET, "404", reqId, "-1", 0, 0, time.Since(t), lambdaReqId}
			return
		}

		// construct lambda store response
		response := &types.Response{
			ResponseWriter: w,
			Cmd:            "get",
			ConnId:         connId,
			ReqId:          reqId,
			ChunkId:        chunk.Id,
			Body:           chunk.Body,
		}
		response.Prepare()
		t3 := time.Now()
		if err := response.Flush(); err != nil {
			log.Error("Error on flush(get key %s): %v", key, err)
			return
		}

		d3 := time.Since(t3)
		dt := time.Since(t)
		log.Debug("Flush duration is %v", d3)
		log.Debug("Total duration is %v", dt)
		log.Debug("Get complete, Key: %s, ConnID:%s, ChunkID:%s", key, connId, chunk.Id)
		dataDeposited.Add(1)
		dataGatherer <- &types.DataEntry{OP_GET, "200", reqId, chunk.Id, 0, d3, dt, lambdaReqId}
	})

	srv.HandleStreamFunc("set", func(w resp.ResponseWriter, c *resp.CommandStream) {
		atomic.AddInt32(&active, 1)
		timeout.Requests++
		extension := lambdaTimeout.TICK_ERROR
		if timeout.Requests > 1 {
			extension = lambdaTimeout.TICK
		}
		defer func() {
			timeout.ResetWithExtension(extension)
			atomic.AddInt32(&active, -1)
		}()

		t := time.Now()
		log.Debug("In SET handler")

		connId, _ := c.NextArg().String()
		reqId, _ := c.NextArg().String()
		chunkId, _ := c.NextArg().String()
		key, _ := c.NextArg().String()
		valReader, err := c.Next()
		if err != nil {
			log.Error("Error on get value reader: %v", err)
			w.AppendErrorf("Error on get value reader: %v", err)
			if err := w.Flush(); err != nil {
				log.Error("Error on flush(error 500): %v", err)
			}
			return
		}
		val, err := valReader.ReadAll()
		if err != nil {
			log.Error("Error on get value: %v", err)
			w.AppendErrorf("Error on get value: %v", err)
			if err := w.Flush(); err != nil {
				log.Error("Error on flush(error 500): %v", err)
			}
			return
		}
		myMap[key] = &types.Chunk{chunkId, val}

		// write Key, clientId, chunkId, body back to server
		response := &types.Response{
			ResponseWriter: w,
			Cmd:            "set",
			ConnId:         connId,
			ReqId:          reqId,
			ChunkId:        chunkId,
		}
		response.Prepare()
		if err := response.Flush(); err != nil {
			log.Error("Error on set::flush(set key %s): %v", key, err)
			return
		}

		log.Debug("Set complete, Key:%s, ConnID: %s, ChunkID: %s, Item length %d", key, connId, chunkId, len(val))
		dataDeposited.Add(1)
		dataGatherer <- &types.DataEntry{OP_SET, "200", reqId, chunkId, 0, 0, time.Since(t), lambdaReqId}
	})

	srv.HandleFunc("data", func(w resp.ResponseWriter, c *resp.Command) {
		timeout.Stop()

		log.Debug("In DATA handler")

		// Wait for data depository.
		dataDeposited.Wait()

		w.AppendBulkString("data")
		w.AppendBulkString(strconv.Itoa(len(dataDepository)))
		for _, entry := range dataDepository {
			format := fmt.Sprintf("%s,%s,%s,%s,%d,%d,%d,%s,%s,%s",
				entry.Op, entry.ReqId, entry.ChunkId, entry.Status,
				entry.Duration, entry.DurationAppend, entry.DurationFlush, hostName, lambdacontext.FunctionName, entry.LambdaReqId)
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
		// No need to close server, it will serve the new connection next time.
		dataDepository = dataDepository[:0]
	})

	srv.HandleFunc("ping", func(w resp.ResponseWriter, c *resp.Command) {
		atomic.AddInt32(&active, 1)

		if IsDone() {
			// If no request comes, ignore. This prevents unexpected pings.
			atomic.AddInt32(&active, -1)
			return
		}

		timeout.ResetWithExtension(lambdaTimeout.TICK_ERROR_EXTEND)
		log.Debug("PING")
		issuePong()
		pong(w)
		atomic.AddInt32(&active, -1)
	})

	srv.HandleFunc("backup", func(w resp.ResponseWriter, c *resp.Command) {
		var err error
		// addr:port
		addrMigrator = c.Arg(0).String()

		// dial to migrator
		srcConn, err = net.Dial("tcp", addrMigrator)
		if err != nil {
			log.Error("connect to migrator failed", err)
			migrateChan <- err
		}

		//trigger migrate lambda
		status, err := trigger(addrMigrator, "functionName")
		if err != nil {
			log.Error("trigger dst lambda err %v", err)
			migrateChan <- err
		}
		if status == STATUSCODE {
			log.Debug("async invoke lambda success %d", status)
		}
		migrateChan <- nil

	})

	// log.Debug("Routings on launching: %d", runtime.NumGoroutine())
	lambda.Start(HandleRequest)
}

func trigger(addr string, name string) (int64, error) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	client := lambdaService.New(sess, &aws.Config{Region: aws.String("us-east-1")})
	event := &protocol.InputEvent{
		MigratorAddr: addr,
		Cmd:          "migrate",
	}
	payload, _ := json.Marshal(event)
	input := &lambdaService.InvokeInput{
		FunctionName:   aws.String(name),
		Payload:        payload,
		InvocationType: aws.String("Event"), /* async invoke*/
	}

	res, err := client.Invoke(input)
	if err != nil {
		log.Error("Error calling migration lambda %s, %v", name, err)
		migrateChan <- err
	}
	return *res.StatusCode, err
}

func initBackup(conn net.Conn) error {
	initBackupWriter := resp.NewResponseWriter(conn)
	// init backup cmd
	initBackupWriter.AppendBulkString("initBackup")

	if err := initBackupWriter.Flush(); err != nil {
		log.Error("Error on initBackup::flush(backup): %v", err)
		return err
	}

	return <-migrateChan
}
