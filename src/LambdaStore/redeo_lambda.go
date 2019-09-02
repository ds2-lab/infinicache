package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/aws"
	awsSession "github.com/aws/aws-sdk-go/aws/session"
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
	"time"

	"github.com/wangaoone/LambdaObjectstore/src/LambdaStore/migrator"
	"github.com/wangaoone/LambdaObjectstore/src/LambdaStore/storage"
	lambdaLife "github.com/wangaoone/LambdaObjectstore/src/LambdaStore/lifetime"
	"github.com/wangaoone/LambdaObjectstore/src/LambdaStore/types"
	protocol "github.com/wangaoone/LambdaObjectstore/src/types"
)

const (
	OP_GET              = "1"
	OP_SET              = "0"
	EXPECTED_GOMAXPROCS = 2
	LIFESPAN            = 5 * time.Minute
	S3BUCKET            = "tianium.default"
)

var (
	ErrProxyClosing               = errors.New("Proxy closed.")

	lambdaConn     net.Conn
	store          types.Storage   = storage.New()
	srv                            = redeo.NewServer(nil)
	log                            = &logger.ColorLogger{ Level: logger.LOG_LEVEL_ALL }
	dataGatherer                   = make(chan *types.DataEntry, 10)
	dataDepository                 = make([]*types.DataEntry, 0, 100)
	dataDeposited  sync.WaitGroup
	lifetime                       = lambdaLife.New(LIFESPAN)
	// Pong limiter prevent pong being sent duplicatedly on launching lambda while a ping arrives
	// at the same time.
	pongLimiter                    = make(chan struct{}, 1)

	mu             sync.RWMutex
	id             uint64
	hostName       string
	lambdaReqId    string
	server         string // Passed from proxy dynamically.
	prefix         string
)

func init() {
	goroutines := runtime.GOMAXPROCS(0)
	if goroutines < EXPECTED_GOMAXPROCS {
		log.Debug("Set GOMAXPROCS to %d (original %d)", EXPECTED_GOMAXPROCS, goroutines)
		runtime.GOMAXPROCS(EXPECTED_GOMAXPROCS)
	} else {
		log.Debug("GOMAXPROCS %d", goroutines)
	}

	cmd := exec.Command("uname", "-a")
	host, err := cmd.CombinedOutput()
	if err != nil {
		log.Debug("cmd.Run() failed with %s\n", err)
	}

	hostName = strings.Split(string(host), " #")[0]
	log.Debug("hostname is: %s", hostName)
}

func getAwsReqId(ctx context.Context) string {
	lc, ok := lambdacontext.FromContext(ctx)
	if ok == false {
		log.Debug("get lambda context failed %v", ok)
	}
	return lc.AwsRequestID
}

func HandleRequest(ctx context.Context, input protocol.InputEvent) error {
	// gorouting start from 3

	// Reset if necessary.
	// This is essential for debugging, and useful if deployment pool is not large enough.
	lifetime.RebornIfDead()
	session := lambdaLife.GetSession()
	defer lambdaLife.ClearSession()

	session.Timeout.SetLogger(log)
	if input.Timeout > 0 {
		deadline, _ := ctx.Deadline()
		session.Timeout.StartWithCalibration(deadline.Add(-time.Duration(input.Timeout) * time.Second))
	} else {
		session.Timeout.Start()
	}
	issuePong()

	// Update global parameters
	prefix = input.Prefix
	lambdaReqId = getAwsReqId(ctx)

	// migration triggered lambda
	if input.Cmd == "migrate" {
		if len(input.Addr) == 0 {
			log.Error("No migrator set.")
			return nil
		}

		mu.Lock()
		if lambdaConn != nil {
			// The connection is not closed on last invocation, reset.
			lambdaConn.Close()
			lambdaConn = nil
			lifetime.Reborn()
		}
		mu.Unlock()

		// connect to migrator
		session.Migrator = migrator.NewClient()
		if err := session.Migrator.Connect(input.Addr); err != nil {
			log.Error("Failed to connect migrator %s: %v", input.Addr, err)
			return nil
		}

		// Send hello
		reader, err := session.Migrator.Send("mhello")
		if err != nil {
			log.Error("Failed to hello source on migrator: %v", err)
			return nil
		}

		// Apply store adapter to coordinate migration and normal requests
		adapter := session.Migrator.GetStoreAdapter(store)
		store = adapter

		// Reader will be avaiable after connecting and source being replaced
		go func(s *lambdaLife.Session) {
			// In-session gorouting
			s.Timeout.Busy()
			defer s.Timeout.DoneBusy()

			s.Migrator.Migrate(reader, store)
			s.Migrator = nil
			store = adapter.Restore()
		}(session)
	}

	// log.Debug("Routings on requesting: %d", runtime.NumGoroutine())
	id = input.Id

	mu.Lock()
	session.Connection = lambdaConn
	mu.Unlock()

	if session.Connection != nil {
		session.Timeout.ResetWithExtension(lambdaLife.TICK_ERROR)

		if len(input.Proxy) == 0 {
			log.Error("No proxy set.")
			return nil
		}

		server = input.Proxy
		log.Debug("Ready to connect %s, id %d", server, id)
		var connErr error
		session.Connection, connErr = net.Dial("tcp", server)
		if connErr != nil {
			log.Error("Failed to connect server %s: %v", server, connErr)
			return connErr
		}
		mu.Lock()
		lambdaConn = session.Connection
		mu.Unlock()
		log.Info("Connection to %v established (%v)", lambdaConn.RemoteAddr(), session.Timeout.Since())

		go func(conn net.Conn) {
			// Cross session gorouting
			err := srv.ServeForeignClient(conn)
			if err != nil && err != io.EOF {
				log.Info("Connection closed: %v", err)
			} else {
				log.Info("Connection closed.")
			}
			conn.Close()

			session := lambdaLife.GetSession()
			mu.Lock()
			defer mu.Unlock()
			if session.Connection == nil {
				// Connection unset, but connection from previous invocation is lost.
				lambdaConn = nil
				lifetime.Reborn()
				return
			} else if session.Connection != conn {
				// Connection changed.
				return
			}

			// Flag destination is ready or we are done.
			lambdaConn = nil
			if session.Migrator != nil {
				session.Migrator.SetReady()
			} else {
				lifetime.Rest()
				session.Done()
			}
		}(session.Connection)
	} else {
		session.Timeout.ResetWithExtension(lambdaLife.TICK_ERROR_EXTEND)
	}
	// append PONG back to proxy on being triggered
	pongHandler(session.Connection)

	// data gathering
	go CollectData(session)

	// timeout control
	Wait(session, lifetime)

	log.Debug("All routing cleared(%d) at %v", runtime.NumGoroutine(), session.Timeout.Since())
	return nil
}

func CollectData(session *lambdaLife.Session) {
	session.Clear.Add(1)
	defer session.Clear.Done()

	for {
		select {
		case <-session.WaitDone():
			return
		case entry := <-dataGatherer:
			dataDepository = append(dataDepository, entry)
			dataDeposited.Done()
		}
	}
}

func Wait(session *lambdaLife.Session, lifetime *lambdaLife.Lifetime) {
	defer session.Clear.Wait()

	select {
	case <-session.WaitDone():
		return
	case <-session.Timeout.C():
		session.Timeout.Halt()

		if lifetime.IsTimeUp() && store.Len() > 0 {
			// Time to migarate
			// TODO: Remove len obligation for store. Store may serve other requests after initiating migration.

			// Initiate migration
			session.Migrator = migrator.NewClient()
			log.Info("Initiate migration.")
			initiator := func() error { return initMigrateHandler(session.Connection) }
			for err := session.Migrator.Initiate(initiator); err != nil; {
				log.Warn("Fail to initiaiate migration: %v", err)
				if err == ErrProxyClosing {
					return
				}

				log.Warn("Retry migration")
				err = session.Migrator.Initiate(initiator)
			}
			log.Debug("Migration initiated.")
		} else {
			session.Done()
			log.Debug("Lambda timeout, return(%v).", session.Timeout.Since())
			return
		}
	}
}



func issuePong() {
	mu.Lock()
	defer mu.Unlock()

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
	mu.Lock()
	defer mu.Unlock()

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
	sess := awsSession.Must(awsSession.NewSessionWithOptions(awsSession.Options{
		SharedConfigState: awsSession.SharedConfigEnable,
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
	dataDepository = dataDepository[:0]
}

func main() {
	// Define handlers
	srv.HandleFunc("get", func(w resp.ResponseWriter, c *resp.Command) {
		session := lambdaLife.GetSession()
		session.Timeout.Busy()
		session.Requests++
		extension := lambdaLife.TICK_ERROR
		if session.Requests > 1 {
			extension = lambdaLife.TICK
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
			session.Timeout.ResetWithExtension(extension)
			session.Timeout.DoneBusy()
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
		session := lambdaLife.GetSession()
		session.Timeout.Busy()
		session.Requests++
		extension := lambdaLife.TICK_ERROR
		if session.Requests > 1 {
			extension = lambdaLife.TICK
		}
		defer func() {
			session.Timeout.ResetWithExtension(extension)
			session.Timeout.DoneBusy()
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
		session := lambdaLife.GetSession()
		session.Timeout.Halt()
		log.Debug("In DATA handler")

		if session.Migrator != nil {
			session.Migrator.SetError(ErrProxyClosing)
			session.Migrator.Close()
			session.Migrator = nil
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
		session.Connection.Close()
		// No need to close server, it will serve the new connection next time.

		// Reset store
		store = storage.New()
	})

	srv.HandleFunc("ping", func(w resp.ResponseWriter, c *resp.Command) {
		session := lambdaLife.GetSession()
		session.Timeout.Busy()
		defer session.Timeout.DoneBusy()

		if session.IsDone() {
			// If no request comes, ignore. This prevents unexpected pings.
			return
		}

		session.Timeout.ResetWithExtension(lambdaLife.TICK_ERROR_EXTEND)
		log.Debug("PING")
		issuePong()
		pong(w)
	})

	srv.HandleFunc("migrate", func(w resp.ResponseWriter, c *resp.Command) {
		session := lambdaLife.GetSession()
		session.Timeout.Halt()
		log.Debug("In MIGRATE handler")

		// addr:port
		addr := c.Arg(0).String()
		deployment := c.Arg(1).String()
		newId, _ := c.Arg(2).Int()
		requestFromProxy := false

		if session.Migrator == nil {
			// Migration initiated by proxy
			requestFromProxy = true
			session.Migrator = migrator.NewClient()
		}

		// dial to migrator
		if err := session.Migrator.Connect(addr); err != nil {
			return
		}

		if err := session.Migrator.TriggerDestination(deployment, &protocol.InputEvent{
			Cmd:   "migrate",
			Id:    uint64(newId),
			Proxy: server,
			Addr:  addr,
			Prefix: prefix,
		}); err != nil {
			return
		}

		// Now, we serve migration connection
		go func(session *lambdaLife.Session) {
			// In session gorouting
			session.Migrator.WaitForMigration(srv)
			// Migration ends or is interrupted.

			// Should be ready if migration ended.
			if session.Migrator.IsReady() {
				// put data to s3 before migration finish
				gatherData(prefix)
				// store = storage.New()
				session.Migrator = nil
				// This is essential for debugging, and useful if deployment pool is not large enough.
				lifetime.Rest()
				session.Done()
			} else if requestFromProxy {
				session.Timeout.Enable()
				session.Timeout.ResetWithExtension(lambdaLife.TICK_ERROR)
			}
		}(session)
	})

	srv.HandleFunc("mhello", func(w resp.ResponseWriter, c *resp.Command) {
		session := lambdaLife.GetSession()
		if session.Migrator == nil {
			log.Error("Migration is not initiated.")
			return
		}

		// Wait for ready, which means connection to proxy is closed and we are safe to proceed.
		err := <-session.Migrator.Ready()
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
