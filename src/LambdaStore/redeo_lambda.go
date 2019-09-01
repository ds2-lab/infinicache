package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
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

	"github.com/wangaoone/LambdaObjectstore/src/LambdaStore/migrator"
	"github.com/wangaoone/LambdaObjectstore/src/LambdaStore/storage"
	lambdaTimeout "github.com/wangaoone/LambdaObjectstore/src/LambdaStore/timeout"
	"github.com/wangaoone/LambdaObjectstore/src/LambdaStore/types"
	protocol "github.com/wangaoone/LambdaObjectstore/src/types"
)

const (
	OP_GET              = "1"
	OP_SET              = "0"
	EXPECTED_GOMAXPROCS = 2
	LIFESPAN            = 60
	S3BUCKET            = "ao.test.data"
)

var (
	startTime  *time.Time
	server     string // Passed from proxy dynamically.
	lambdaConn net.Conn
	srv                      = redeo.NewServer(nil)
	store      types.Storage = storage.New()
	isFirst                  = true
	log                      = &logger.ColorLogger{
		Level: logger.LOG_LEVEL_WARN,
	}
	dataGatherer   = make(chan *types.DataEntry, 10)
	dataDepository = make([]*types.DataEntry, 0, 100)
	dataDeposited  sync.WaitGroup
	timeout        = lambdaTimeout.New(0)
	// Pong limiter prevent pong being sent duplicatedly on launching lambda while a ping arrives
	// at the same time.
	pongLimiter = make(chan struct{}, 1)

	active      int32
	mu          sync.RWMutex
	done        chan struct{}
	id          uint64
	hostName    string
	lambdaReqId string
	migrClient  *migrator.Client
	prefix      string

	ErrProxyClosing = errors.New("Proxy closed.")
)

func init() {
	start := time.Now()
	startTime = &start

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
	prefix = input.Prefix
	if startTime == nil {
		// Reset if necessary.
		// This is essential for debugging, and useful if deployment pool is not large enough.
		start := time.Now()
		startTime = &start
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

	// get lambda invoke reqId
	lambdaReqId = getAwsReqId(ctx)

	// migration triggered lambda
	if input.Cmd == "migrate" {
		if len(input.Addr) == 0 {
			log.Error("No migrator set.")
			return nil
		}

		// connect to migrator
		migrClient = migrator.NewClient()
		if err := migrClient.Connect(input.Addr); err != nil {
			log.Error("Failed to connect migrator %s: %v", input.Addr, err)
			return nil
		}

		// Send hello
		reader, err := migrClient.Send("mhello")
		if err != nil {
			log.Error("Failed to hello source on migrator: %v", err)
			return nil
		}

		// Apply store adapter to coordinate migration and normal requests
		adapter := migrClient.GetStoreAdapter(store)
		store = adapter

		// Reader will be avaiable after connecting and source being replaced
		go func() {
			atomic.AddInt32(&active, 1)
			defer atomic.AddInt32(&active, -1)

			migrClient.Migrate(reader, store)
			migrClient = nil
			store = adapter.Restore()
		}()
	}

	// log.Debug("Routings on requesting: %d", runtime.NumGoroutine())
	id = input.Id

	if isFirst == true {
		timeout.ResetWithExtension(lambdaTimeout.TICK_ERROR)

		if len(input.Proxy) == 0 {
			log.Error("No proxy set.")
			return nil
		}

		server = input.Proxy
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

			// Flag destination is ready or we are done.
			if migrClient != nil {
				migrClient.SetReady()
			} else {
				store = storage.New()
				Done()
			}
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
				} else if time.Since(*startTime).Minutes() >= LIFESPAN && store.Len() > 0 {
					// TODO: Remove len obligation for store. Store may serve other requests after initiating migration.
					// Time to migarate
					// Disable timer so Reset will not work.
					timeout.Disable()
					mu.Unlock()

					// Initiate migration
					migrClient = migrator.NewClient()
					log.Info("Initiate migration.")
					initiator := func() error { return initMigrateHandler(lambdaConn) }
					for err := migrClient.Initiate(initiator); err != nil; {
						log.Warn("Fail to initiaiate migration: %v", err)
						if err == ErrProxyClosing {
							return
						}

						log.Warn("Retry migration")
						err = migrClient.Initiate(initiator)
					}

					// No more timeout, just wait for done
					log.Debug("Migration initiated.")
					break
				} else {
					doneLocked()
					mu.Unlock()
					log.Debug("Lambda timeout, return(%v).", timeout.Since())
					return
				}
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

	// Migrating? Wait it is over
	if migrClient != nil {
		return
	}

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
	writer := resp.NewResponseWriter(conn)
	pong(writer)
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

func initMigrateHandler(conn net.Conn) error {
	writer := resp.NewResponseWriter(conn)
	// init backup cmd
	writer.AppendBulkString("initMigrate")
	return writer.Flush()
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

func remotePut(bucket string, k string, f string) {
	// The session the S3 Uploader will use
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Config:            aws.Config{Region: aws.String("us-east-1")},
	}))

	// Create an uploader with the session and default options
	uploader := s3manager.NewUploader(sess)

	// Upload input parameters
	file := strings.NewReader(f)

	key := fmt.Sprintf("%s-%s", k, lambdacontext.FunctionName)

	upParams := &s3manager.UploadInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   file,
	}
	// Perform an upload.
	result, err := uploader.Upload(upParams)
	if err != nil {
		log.Error("err is ", err)
	}
	log.Debug("upload to S3 res: ", result.Location)
}

func gatherData(prefix string) {
	var dat bytes.Buffer
	for _, entry := range dataDepository {
		format := fmt.Sprintf("%s,%s,%s,%s,%d,%d,%d,%s,%s,%s\n",
			entry.Op, entry.ReqId, entry.ChunkId, entry.Status,
			entry.Duration, entry.DurationAppend, entry.DurationFlush, hostName, lambdacontext.FunctionName, entry.LambdaReqId)
		dat.WriteString(format)
		//w.AppendBulkString(format)

		//w.AppendBulkString(entry.Op)
		//w.AppendBulkString(entry.Status)
		//w.AppendBulkString(entry.ReqId)
		//w.AppendBulkString(entry.ChunkId)
		//w.AppendBulkString(entry.DurationAppend.String())
		//w.AppendBulkString(entry.DurationFlush.String())
		//w.AppendBulkString(entry.Duration.String())
	}
	remotePut(S3BUCKET, prefix, dat.String())
}

func main() {
	// Define handlers
	srv.HandleFunc("get", func(w resp.ResponseWriter, c *resp.Command) {
		atomic.AddInt32(&active, 1)
		timeout.Requests++
		extension := lambdaTimeout.TICK_ERROR
		if timeout.Requests > 1 {
			extension = lambdaTimeout.TICK
		}
		var respError *types.ResponseError

		t := time.Now()
		log.Debug("In GET handler")

		connId := c.Arg(0).String()
		reqId := c.Arg(1).String()
		key := c.Arg(3).String()

		defer func() {
			if respError != nil {
				log.Warn("Failed to get %s: %v", key, respError)
				w.AppendErrorf("Failed to get %s: %v", key, respError)
				if err := w.Flush(); err != nil {
					log.Error("Error on flush: %v", err)
				}
				dataDeposited.Add(1)
				dataGatherer <- &types.DataEntry{OP_GET, respError.Status(), reqId, "-1", 0, 0, time.Since(t), lambdaReqId}
			}
			timeout.ResetWithExtension(extension)
			atomic.AddInt32(&active, -1)
		}()

		//val, err := myCache.Get(key)
		//if err == false {
		//	log.Debug("not found")
		//}
		t2 := time.Now()
		chunkId, stream, err := store.Get(key)
		d2 := time.Since(t2)

		if err == nil {
			// construct lambda store response
			response := &types.Response{
				ResponseWriter: w,
				Cmd:            c.Name,
				ConnId:         connId,
				ReqId:          reqId,
				ChunkId:        chunkId,
				BodyStream:     stream,
			}
			response.Prepare()

			t3 := time.Now()
			if err := response.Flush(); err != nil {
				log.Error("Error on flush(get key %s): %v", key, err)
				return
			}
			d3 := time.Since(t3)

			dt := time.Since(t)
			log.Debug("Streaming duration is %v", d3)
			log.Debug("Total duration is %v", dt)
			log.Debug("Get complete, Key: %s, ConnID:%s, ChunkID:%s", key, connId, chunkId)
			dataDeposited.Add(1)
			dataGatherer <- &types.DataEntry{OP_GET, "200", reqId, chunkId, d2, d3, dt, lambdaReqId}
		} else if err == types.ErrNotFound {
			// Not found
			respError = types.NewResponseError(404, err)
		} else {
			respError = types.NewResponseError(500, err)
		}
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
		store.Set(key, chunkId, val)

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

		if migrClient != nil {
			migrClient.SetError(ErrProxyClosing)
			migrClient.Close()
			migrClient = nil
		}

		// Wait for data depository.
		dataDeposited.Wait()
		// put DATA to s3
		gatherData(prefix)

		w.AppendBulkString("data")
		w.AppendBulkString("OK")
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

	srv.HandleFunc("migrate", func(w resp.ResponseWriter, c *resp.Command) {
		atomic.AddInt32(&active, 1)
		defer atomic.AddInt32(&active, -1)

		log.Debug("In MIGRATE handler")

		timeout.Disable()
		timeout.Stop()

		// addr:port
		addr := c.Arg(0).String()
		deployment := c.Arg(1).String()
		newId, _ := c.Arg(2).Int()

		if migrClient == nil {
			// Migration initiated by proxy
			migrClient = migrator.NewClient()
		}

		// dial to migrator
		if err := migrClient.Connect(addr); err != nil {
			return
		}

		if err := migrClient.TriggerDestination(deployment, &protocol.InputEvent{
			Cmd:   "migrate",
			Id:    uint64(newId),
			Proxy: server,
			Addr:  addr,
			Prefix: prefix,
		}); err != nil {
			return
		}

		// Now, we serve migration connection
		go func() {
			migrClient.WaitForMigration(srv)
			// Migration ends or is interrupted.

			// Should be ready if migration ended.
			if migrClient.IsReady() {
				// FIXME: Reset startTime for reusing.
				// This is essential for debugging, and useful if deployment pool is not large enough.
				startTime = nil
				// Reset client and end lambda
				migrClient = nil
				// put data to s3 before migration finish
				gatherData(prefix)
				store = storage.New()
				Done()
			}
		}()
	})

	srv.HandleFunc("mhello", func(w resp.ResponseWriter, c *resp.Command) {
		if migrClient == nil {
			log.Error("Migration is not initiated.")
			return
		}

		// Wait for ready, which means connection to proxy is closed and we are safe to proceed.
		err := <-migrClient.Ready()
		if err != nil {
			return
		}

		// Send key list by access time
		w.AppendBulkString("mhello")
		w.AppendBulkString(strconv.Itoa(store.Len()))
		for key := range store.Keys() {
			w.AppendBulkString(key)
		}
		if err := w.Flush(); err != nil {
			log.Error("Error on mhello::flush: %v", err)
			return
		}
	})

	// log.Debug("Routings on launching: %d", runtime.NumGoroutine())
	lambda.Start(HandleRequest)
}
