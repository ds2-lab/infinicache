
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/neboduus/infinicache/node/common/logger"
	"github.com/mason-leap-lab/redeo"
	"github.com/mason-leap-lab/redeo/resp"

	//	"github.com/wangaoone/s3gof3r"
	"io"
	"net"
	"runtime"
	"strconv"
	"sync"
	"time"

	protocol "github.com/neboduus/infinicache/node/common/types"

	// We want to remove the collector because it connects to AWS
	"github.com/neboduus/infinicache/node/lambda/collector"

	lambdaLife "github.com/neboduus/infinicache/node/lambda/lifetime"
	"github.com/neboduus/infinicache/node/lambda/migrator"
	"github.com/neboduus/infinicache/node/lambda/storage"
	"github.com/neboduus/infinicache/node/lambda/types"
)

const (
	EXPECTED_GOMAXPROCS = 2
	LIFESPAN            = 5 * time.Minute
)

var (
	// Track how long the store has lived, migration is required before timing up.
	lifetime = lambdaLife.New(LIFESPAN)

	// Data storage
	store   types.Storage = storage.New()
	storeId uint64

	// Proxy that links stores as a system
	proxy     string // Passed from proxy dynamically.
	proxyConn net.Conn
	srv       = redeo.NewServer(nil) // Serve requests from proxy

	mu  sync.RWMutex
	log = &logger.ColorLogger{Level: logger.LOG_LEVEL_WARN}
	// Pong limiter prevent pong being sent duplicatedly on launching lambda while a ping arrives
	// at the same time.
	pongLimiter = make(chan struct{}, 1)

	reqCounter = 0
)

func init() {
	goroutines := runtime.GOMAXPROCS(0)
	if goroutines < EXPECTED_GOMAXPROCS {
		log.Debug("Set GOMAXPROCS to %d (original %d)", EXPECTED_GOMAXPROCS, goroutines)
		runtime.GOMAXPROCS(EXPECTED_GOMAXPROCS)
	} else {
		log.Debug("GOMAXPROCS %d", goroutines)
	}
}

// MigrationToDo: generate new unique ID per request concat(nodeID,reqCount)
func getAwsReqId() string {
	/*	lc, ok := lambdacontext.FromContext(ctx)
		if ok == false {
			log.Debug("get lambda context failed %v", ok)
		}*/
	reqCounter++
	return strconv.Itoa(reqCounter)
}

type server struct{}

// this is aws Lambda handler signature and includes the code which will be executed
// MigrationToDo: Remove the handler and keep the function as a server
func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var input protocol.InputEvent

	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		log.Error("Unable to decode request body: %v, %v", err, r.Body)
	}else{
		log.Debug("r.Body: %v",input)
	}

	// gorouting start from 3

	// Reset if necessary.
	// This is essential for debugging, and useful if deployment pool is not 3large enough.
	lifetime.RebornIfDead()
	session := lambdaLife.GetOrCreateSession()
	session.Id = getAwsReqId()
	defer lambdaLife.ClearSession()

	session.Timeout.SetLogger(log)
	if input.Timeout > 0 {
		deadline := time.Now().Add(100*time.Second)
		session.Timeout.StartWithCalibration(deadline.Add(-time.Duration(input.Timeout) * time.Second))
	} else {
		session.Timeout.Start()
	}
	issuePong()

	// Update global parameters
	collector.Prefix = input.Prefix
	log.Level = input.Log

	// log.Info("New lambda invocation: %v", input.Cmd)

	// migration triggered lambda
	if input.Cmd == "migrate" {
		// collector.Send(&types.DataEntry{Op: types.OP_MIGRATION, Session: session.Id})

		if len(input.Addr) == 0 {
			log.Error("No migrator set.")
			return
		}

		mu.Lock()
		if proxyConn != nil {
			// The connection is not closed on last invocation, reset.
			proxyConn.Close()
			proxyConn = nil
			lifetime.Reborn()
		}
		mu.Unlock()

		// connect to migrator
		session.Migrator = migrator.NewClient()
		if err := session.Migrator.Connect(input.Addr); err != nil {
			log.Error("Failed to connect migrator %s: %v", input.Addr, err)
			return
		}

		// Send hello
		reader, err := session.Migrator.Send("mhello", nil)
		if err != nil {
			log.Error("Failed to hello source on migrator: %v", err)
			return
		}

		// Apply store adapter to coordinate migration and normal requests
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

	mu.Lock()
	session.Connection = proxyConn
	mu.Unlock()

	if session.Connection == nil {
		if len(input.Proxy) == 0 {
			log.Error("No proxy set.")
			return
		}

		storeId = input.Id
		proxy = input.Proxy
		log.Debug("Ready to connect %s, id %d", proxy, storeId)
		var connErr error
		session.Connection, connErr = net.Dial("tcp", proxy)


		if connErr != nil {
			log.Error("Failed to connect proxy %s: %v", proxy, connErr)
			// return connErr
			return
		}
		mu.Lock()
		proxyConn = session.Connection
		mu.Unlock()
		log.Info("Connection to %v established (%v)", proxyConn.RemoteAddr(), session.Timeout.Since())

		go func(conn net.Conn) {
			// Cross session gorouting
			err := srv.ServeForeignClient(conn)
			if err != nil && err != io.EOF {
				log.Info("Connection closed: %v", err)
			} else {
				log.Info("Connection closed.")
			}
			conn.Close()

			session := lambdaLife.GetOrCreateSession()
			mu.Lock()
			defer mu.Unlock()
			if session.Connection == nil {
				// Connection unset, but connection from previous invocation is lost.
				proxyConn = nil
				lifetime.Reborn()
				return
			} else if session.Connection != conn {
				// Connection changed.
				return
			}

			// Flag destination is ready or we are done.
			proxyConn = nil
			if session.Migrator != nil {
				session.Migrator.SetReady()
			} else {
				lifetime.Rest()
				session.Done()
			}
		}(session.Connection)
	}
	if input.Cmd == "warmup" {
		session.Timeout.ResetWithExtension(lambdaLife.TICK_ERROR)
		// collector.Send(&types.DataEntry{Op: types.OP_WARMUP, Session: session.Id})
	} else {
		session.Timeout.ResetWithExtension(lambdaLife.TICK_ERROR_EXTEND)
	}
	// append PONG back to proxy on being triggered
	pongHandler(session.Connection)

	// data gathering
	// go collector.Collect(session)

	// timeout control
	Wait(session, lifetime)

	log.Debug("All routing cleared(%d) at %v", runtime.NumGoroutine(), session.Timeout.Since())
	return
}

func Wait(session *lambdaLife.Session, lifetime *lambdaLife.Lifetime) {
	defer session.Clear.Wait()

	select {
	case <-session.WaitDone():
		return
	case <-session.Timeout.C():
		// There's no turning back.
		session.Timeout.Halt()

		if lifetime.IsTimeUp() && store.Len() > 0 {
			// Time to migrate
			// Check of number of keys in store is necessary. As soon as there is any value
			// in the store and time up, we should start migration.

			// Initiate migration
			session.Migrator = migrator.NewClient()
			log.Info("Initiate migration.")
			initiator := func() error { return initMigrateHandler(session.Connection) }
			for err := session.Migrator.Initiate(initiator); err != nil; {
				log.Warn("Fail to initiate migration: %v", err)
				if err == types.ErrProxyClosing {
					return
				}

				log.Warn("Retry migration")
				err = session.Migrator.Initiate(initiator)
			}
			log.Debug("Migration initiated.")
		} else {
			byeHandler(session.Connection)
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

func pongHandler(conn net.Conn) error {
	writer := resp.NewResponseWriter(conn)
	return pong(writer)
}

func pong(w resp.ResponseWriter) error {
	mu.Lock()
	defer mu.Unlock()

	select {
	case <-pongLimiter:
		// Quota avaiable or abort.
	default:
		return nil
	}

	w.AppendBulkString("pong")
	w.AppendInt(int64(storeId))
	if err := w.Flush(); err != nil {
		log.Error("Error on PONG flush: %v", err)
		return err
	}

	return nil
}

func initMigrateHandler(conn net.Conn) error {
	writer := resp.NewResponseWriter(conn)
	// init backup cmd
	writer.AppendBulkString("initMigrate")
	return writer.Flush()
}

func byeHandler(conn net.Conn) error {
	writer := resp.NewResponseWriter(conn)
	// init backup cmd
	writer.AppendBulkString("bye")
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

func main() {
	// Define handlers

	srv.HandleStreamFunc("set", func(w resp.ResponseWriter, c *resp.CommandStream) {
		session := lambdaLife.GetSession()
		session.Timeout.Busy()
		session.Requests++
		extension := lambdaLife.TICK_ERROR
		if session.Requests > 1 {
			extension = lambdaLife.TICK
		}
		defer session.Timeout.DoneBusyWithReset(extension)

		// t := time.Now()
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
		// val, err := valReader.ReadAll()
		// if err != nil {
		// 	log.Error("Error on get value: %v", err)
		// 	w.AppendErrorf("Error on get value: %v", err)
		// 	if err := w.Flush(); err != nil {
		// 		log.Error("Error on flush(error 500): %v", err)
		// 	}
		// 	return
		// }
		err = store.SetStream(key, chunkId, valReader)
		if err != nil {
			log.Error("%v", err)
			w.AppendErrorf("%v", err)
			if err := w.Flush(); err != nil {
				log.Error("Error on flush(error 500): %v", err)
			}
			return
		}

		// write Key, clientId, chunkId, body back to proxy
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

		log.Debug("Set complete, Key:%s, ConnID: %s, ChunkID: %s", key, connId, chunkId)
		// collector.Send(&types.DataEntry{types.OP_SET, "200", reqId, chunkId, 0, 0, time.Since(t), session.Id})
	})

	srv.HandleFunc("get", func(w resp.ResponseWriter, c *resp.Command) {
		session := lambdaLife.GetSession()
		session.Timeout.Busy()
		session.Requests++
		extension := lambdaLife.TICK_ERROR
		if session.Requests > 1 {
			extension = lambdaLife.TICK
		}
		defer session.Timeout.DoneBusyWithReset(extension)

		t := time.Now()
		log.Debug("In GET handler")

		connId := c.Arg(0).String()
		reqId := c.Arg(1).String()
		key := c.Arg(3).String()

		//val, err := myCache.Get(key)
		//if err == false {
		//	log.Debug("not found")
		//}
		// t2 := time.Now()
		chunkId, stream, err := store.GetStream(key)
		//d2 := time.Since(t2)

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
			// collector.Send(&types.DataEntry{types.OP_GET, "200", reqId, chunkId, d2, d3, dt, session.Id})
		} else {
			var respError *types.ResponseError
			if err == types.ErrNotFound {
				// Not found
				respError = types.NewResponseError(404, err)
			} else {
				respError = types.NewResponseError(500, err)
			}

			log.Warn("Failed to get %s: %v", key, respError)
			w.AppendErrorf("Failed to get %s: %v", key, respError)
			if err := w.Flush(); err != nil {
				log.Error("Error on flush: %v", err)
			}
			// collector.Send(&types.DataEntry{types.OP_GET, respError.Status(), reqId, "-1", 0, 0, time.Since(t), session.Id})
		}
	})

	srv.HandleFunc("mkset", func(w resp.ResponseWriter, c *resp.Command) {
		session := lambdaLife.GetSession()
		session.Timeout.Busy()
		session.Requests++
		extension := lambdaLife.TICK_ERROR
		if session.Requests > 1 {
			extension = lambdaLife.TICK
		}
		defer session.Timeout.DoneBusyWithReset(extension)

		// t := time.Now()
		log.Debug("In MKSET handler")

		connId := c.Arg(0).String()
		reqId := c.Arg(1).String()
		chunkId := c.Arg(2).String()
		key := c.Arg(3).String()
		//values, _ := c.Arg(4).Int()
		var lowLevelKeys []string

		for i:=5; i<c.ArgN(); i=i+2 {
			lowLevelKey := c.Arg(i).String()
			lowLevelKeys = append(lowLevelKeys, lowLevelKey)
			chunkKey := fmt.Sprintf("%s@%s", key, lowLevelKey)
			value := c.Arg(i+1).Bytes()
			err := store.Set(chunkKey, chunkKey, value)
			if err != nil {
				log.Error("%v", err)
				w.AppendErrorf("%v", err)
				if err := w.Flush(); err != nil {
					log.Error("Error on flush(error 500): %v", err)
				}
				return
			}else{
				log.Debug("SET OK < %s , %s >", chunkKey, string(value))
			}
		}

		// write Key, clientId, chunkId, body back to proxy
		response := &types.Response{
			ResponseWriter: w,
			Cmd:            "mkset",
			ConnId:         connId,
			ReqId:          reqId,
			ChunkId:        chunkId,
		}
		response.Prepare()
		if err := response.Flush(); err != nil {
			log.Error("Error on mkSet::flush(set key %s): %v", key, err)
			return
		}

		log.Debug("mkSet complete, Key:%s, ConnID: %s, ChunkID: %s, LowLevelKeys: %s", key, connId, chunkId, lowLevelKeys)
		// collector.Send(&types.DataEntry{types.OP_SET, "200", reqId, chunkId, 0, 0, time.Since(t), session.Id})
	})

	srv.HandleFunc("mkget", func(w resp.ResponseWriter, c *resp.Command) {
		session := lambdaLife.GetSession()
		session.Timeout.Busy()
		session.Requests++
		extension := lambdaLife.TICK_ERROR
		if session.Requests > 1 {
			extension = lambdaLife.TICK
		}
		defer session.Timeout.DoneBusyWithReset(extension)

		t := time.Now()
		log.Debug("In MKGET handler")

		connId := c.Arg(0).String()
		reqId := c.Arg(1).String()
		chunkId := c.Arg(2).String()
		key := c.Arg(4).String()
		// lowLevelKeysN, _ := c.Arg(5).Int()
		lowLevelKeyValuePairs := make(map[string][]byte)

		var failedLowLevelKeys []string
		var tErr error

		for i:=6;i<c.ArgN();i++{
			lowLevelKey := c.Arg(i).String()
			lowLevelKey = fmt.Sprintf("%s@%s", key, lowLevelKey)
			k, value, err := store.Get(lowLevelKey)
			lowLevelKeyValuePairs[k] = value
			if err != nil {
				failedLowLevelKeys = append(failedLowLevelKeys, lowLevelKey)
				tErr = err
				log.Debug("Not Found < %s >", lowLevelKey)
			}else{
				log.Debug("Found < %s , %s >", lowLevelKey, value)
			}
		}

		if tErr != nil {
			var respError *types.ResponseError
			// Not found
			respError = types.NewResponseError(404, types.ErrNotFound)
			log.Warn("404: Failed to get %s, specifically %s: %v", key, failedLowLevelKeys, respError)
			w.AppendErrorf("404: Failed to get %s, specifically %s: %v", key, failedLowLevelKeys, respError)
			// collector.Send(&types.DataEntry{types.OP_GET, respError.Status(), reqId, "-1", 0, 0, time.Since(t), session.Id})
			if err := w.Flush(); err != nil {
				log.Error("Error on flush: %v", err)
			}
			return
		}

		// construct lambda store response
		response := &types.Response{
			ResponseWriter: w,
			Cmd:            c.Name,
			ConnId:         connId,
			ReqId:          reqId,
			ChunkId: chunkId,
			LowLevelKeyValuePairs: lowLevelKeyValuePairs,
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
		log.Debug("mkGet complete, Key: %s, ConnID:%s, ChunkIDs:[%s]", key, connId, printKeys(lowLevelKeyValuePairs))
		// collector.Send(&types.DataEntry{types.OP_GET, "200", reqId, chunkId, d2, d3, dt, session.Id})
	})

	srv.HandleFunc("del", func(w resp.ResponseWriter, c *resp.Command) {
		session := lambdaLife.GetSession()
		session.Timeout.Busy()
		session.Requests++
		extension := lambdaLife.TICK_ERROR
		if session.Requests > 1 {
			extension = lambdaLife.TICK
		}
		defer session.Timeout.DoneBusyWithReset(extension)

		//t := time.Now()
		log.Debug("In Del Handler")

		connId := c.Arg(0).String()
		reqId := c.Arg(1).String()
		chunkId := c.Arg(2).String()
		key := c.Arg(3).String()

		err := store.Del(key, chunkId)
		if err == nil {
			// write Key, clientId, chunkId, body back to proxy
			response := &types.Response{
				ResponseWriter: w,
				Cmd:            "del",
				ConnId:         connId,
				ReqId:          reqId,
				ChunkId:        chunkId,
			}
			response.Prepare()
			if err := response.Flush(); err != nil {
				log.Error("Error on del::flush(set key %s): %v", key, err)
				return
			}
		} else {
			var respError *types.ResponseError
			if err == types.ErrNotFound {
				// Not found
				respError = types.NewResponseError(404, err)
			} else {
				respError = types.NewResponseError(500, err)
			}

			log.Warn("Failed to del %s: %v", key, respError)
			w.AppendErrorf("Failed to del %s: %v", key, respError)
			if err := w.Flush(); err != nil {
				log.Error("Error on flush: %v", err)
			}
		}

	})
	srv.HandleFunc("data", func(w resp.ResponseWriter, c *resp.Command) {
		session := lambdaLife.GetSession()
		session.Timeout.Halt()
		log.Debug("In DATA handler")

		if session.Migrator != nil {
			session.Migrator.SetError(types.ErrProxyClosing)
			session.Migrator.Close()
			session.Migrator = nil
		}

		// put DATA to s3
		// collector.Save(lifetime)

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
		if session == nil {
			// Possibilities are ping may comes after HandleRequest returned
			log.Debug("PING ignored: session ended.")
			return
		} else if !session.Timeout.ResetWithExtension(lambdaLife.TICK_ERROR_EXTEND) && !session.IsMigrating() {
			// Failed to extend timeout, do nothing and prepare to return from lambda.
			log.Debug("PING ignored: timeout extension denied.")
			return
		}

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

		if !session.IsMigrating() {
			// Migration initiated by proxy
			requestFromProxy = true
			session.Migrator = migrator.NewClient()
		}

		// dial to migrator
		if err := session.Migrator.Connect(addr); err != nil {
			return
		}

		if err := session.Migrator.TriggerDestination(deployment, &protocol.InputEvent{
			Cmd:    "migrate",
			Id:     uint64(newId),
			Proxy:  proxy,
			Addr:   addr,
			Prefix: collector.Prefix,
			Log:    log.GetLevel(),
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
				// collector.Save(lifetime)

				// This is essential for debugging, and useful if deployment pool is not large enough.
				lifetime.Rest()
				// Keep or not? It is a problem.
				// KEEP: MUST if migration is used for backup
				// DISCARD: SHOULD if to be reused after migration.
				// store = storage.New()

				// Close session
				session.Migrator = nil
				session.Done()
			} else if requestFromProxy {
				session.Migrator = nil
				session.Timeout.Restart(lambdaLife.TICK_ERROR)
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

		delList := make([]string, 0, 2*store.Len())
		getList := delList[store.Len():store.Len()]
		for key := range store.Keys() {
			_, _, err := store.Get(key)
			if err == types.ErrNotFound {
				delList = append(delList, key)
			} else {
				getList = append(getList, key)
			}
		}

		for _, key := range delList {
			w.AppendBulkString(fmt.Sprintf("%d%s", types.OP_DEL, key))
		}
		for _, key := range getList {
			w.AppendBulkString(fmt.Sprintf("%d%s", types.OP_GET, key))
		}

		if err := w.Flush(); err != nil {
			log.Error("Error on mhello::flush: %v", err)
			return
		}
	})

	port := "8080"
	s := &server{}
	http.Handle("/", s)
	// http.HandleFunc("/", HandleRequest)

	err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
	if err != nil {
		log.Info("ERROR while opening http", port)
	}

/*	go func() {
		err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
		if err != nil {
			log.Info("ERROR while opening http", port)
		}
	}()*/
	log.Info("helloworld: listening on port %s", port)


}

func printKeys(data map[string][]byte) string{
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}

	return strings.Join(keys, ",")
}