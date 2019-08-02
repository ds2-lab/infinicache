package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/ScottMansfield/nanolog"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/wangaoone/redeo"
	"github.com/wangaoone/redeo/resp"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"github.com/wangaoone/LambdaObjectstore/lib/logger"
)

const MaxLambdaStores = 14
const LambdaStoreName = "LambdaStore"

var (
	replica       = flag.Bool("replica", true, "Enable lambda replica deployment")
	isPrint       = flag.Bool("isPrint", false, "Enable log printing")
	prefix        = flag.String("prefix", "log", "log file prefix")
	dataCollected sync.WaitGroup
	log           = &logger.ColorLogger{
		Level: logger.LOG_LEVEL_INFO,
	}
)

var (
	lambdaLis net.Listener
	//cMap      = make(map[int]chan interface{}) // client channel mapping table
	//cMap      = hashmap.New(1024 * 1024)
	cMap      = make([]chan interface{}, 1024*1024)
	filePath  = "/tmp/pidLog.txt"
	timeStamp = time.Now()
	reqMap    = make(map[string]*dataEntry)
	logMu     sync.Mutex
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

func nanoLog(handle nanolog.Handle, args ...interface{}) error {
	timeStamp = time.Now()
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

func logCreate() {
	// get local time
	//location, _ := time.LoadLocation("EST")
	// Set up nanoLog writer
	//nanoLogout, err := os.Create("/tmp/proxy/" + *prefix + "_proxy.clog")
	nanoLogout, err := os.Create(*prefix + "_proxy.clog")
	if err != nil {
		panic(err)
	}
	err = nanolog.SetWriter(nanoLogout)
	if err != nil {
		panic(err)
	}
}

func main() {
	done := make(chan struct{})
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL, syscall.SIGABRT)
	//signal.Notify(sig, syscall.SIGKILL)
	//signal.Notify(sig, syscall.SIGINT)
	flag.Parse()
	// CPU profiling by default
	//defer profile.Start().Stop()
	// init log
	logCreate()
	if *isPrint {
		log.Level = logger.LOG_LEVEL_ALL
	}

	log.Info("======================================")
	log.Info("replica: %v || isPrint: %v", *replica, *isPrint)
	log.Info("======================================")
	clientLis, err := net.Listen("tcp", ":6378")
	if err != nil {
		log.Error("Failed to listen clients: %v", err)
		os.Exit(1)
		return
	}
	lambdaLis, err = net.Listen("tcp", ":6379")
	if err != nil {
		log.Error("Failed to listen lambdas: %v", err)
		os.Exit(1)
		return
	}
	log.Info("Start listening to clients(port 6378) and lambdas(port 6379)")
	// initial proxy and lambda server
	srv := redeo.NewServer(nil)
	lambdaSrv := redeo.NewServer(nil)

	// initial lambda store group
	group := initial(lambdaSrv)

	err = ioutil.WriteFile(filePath, []byte(fmt.Sprintf("%d", os.Getpid())), 0660)
	if err != nil {
		log.Warn("Failed to write PID: %v", err)
	}
	log.Info("Proxy for lambda store is ready!")

	// Log goroutine
	//defer t.Stop()
	go func() {
		t := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-sig:
				log.Info("Receive signal, killing server...")
				t.Stop()
				if err := nanolog.Flush(); err != nil {
					log.Error("Failed to save data: %v", err)
				}

				// Collect data
				for _, node := range group.Arr {
					node.W.WriteCmdString("data")
					err := node.W.Flush()
					if err != nil {
						log.Warn("Failed to submit data request: %v", err)
						continue
					}
					dataCollected.Add(1)
				}
				log.Info("Waiting data from Lambda")
				dataCollected.Wait()
				if err := nanolog.Flush(); err != nil {
					log.Error("Failed to save data from lambdas: %v", err)
				}

				err := os.Remove(filePath)
				if err != nil {
					log.Error("Failed to remove PID: %v", err)
				}

				lambdaLis.Close()
				clientLis.Close()
				close(done)
				log.Info("Server closed.")
				// Collect data

				return
			case <-t.C:
				if time.Since(timeStamp) >= 10*time.Second {
					if err := nanolog.Flush(); err != nil {
						log.Warn("Failed to save data: %v", err)
					}
				}
			}
		}
	}()

	// Start serving (blocking)
	err = srv.MyServe(clientLis, cMap, group, nanoLog, done)
	if err != nil {
		log.Error("Failed to serve clients: %v", err)
		os.Exit(1)
		return
	}

	os.Exit(0)
}

// initial lambda group
func initial(lambdaSrv *redeo.Server) redeo.Group {
	group := redeo.Group{Arr: make([]*redeo.LambdaInstance, MaxLambdaStores), MemCounter: 0}
	if *replica == true {
		for i := range group.Arr {
			node := newLambdaInstance(LambdaStoreName)
			log.Info("[No.%d replication lambda store has registered.]", i)
			// register lambda instance to group
			group.Arr[i] = node
			node.Alive = true
			go lambdaTrigger(node)
			// start a new server to receive conn from lambda store
			node.Cn = lambdaSrv.Accept(lambdaLis)
			log.Info("[start a new conn, lambda store has connected: %v]", node.Cn.RemoteAddr())
			// wrap writer and reader
			node.W = resp.NewRequestWriter(node.Cn)
			node.R = resp.NewResponseReader(node.Cn)
			// lambda handler
			go lambdaHandler(node)
			// lambda facing peeking response type
			go LambdaPeek(node)
			log.Info("[%v]", node.Alive)
		}
	} else {
		for i := range group.Arr {
			node := newLambdaInstance("Node" + strconv.Itoa(i))
			log.Info("[%s lambda store has registered]", node.Name)
			// register lambda instance to group
			group.Arr[i] = node
			node.Alive = true
			go lambdaTrigger(node)
			// start a new server to receive conn from lambda store
			node.Cn = lambdaSrv.Accept(lambdaLis)
			log.Info("[start a new conn, lambda store has connected: %v]", node.Cn.RemoteAddr())
			// wrap writer and reader
			node.W = resp.NewRequestWriter(node.Cn)
			node.R = resp.NewResponseReader(node.Cn)
			// lambda handler
			go lambdaHandler(node)
			// lambda facing peeking response type
			go LambdaPeek(node)
			log.Info("[%v]", node.Alive)
		}
	}
	return group
}

// create new lambda instance
func newLambdaInstance(name string) *redeo.LambdaInstance {
	return &redeo.LambdaInstance{
		Name:  name,
		Alive: false,
		C:     make(chan *redeo.ServerReq, 1024*1024),
	}
}

// blocking on lambda peek Type
// lambda handle incoming lambda store response
//
// field 0 : conn id
// field 1 : req id
// field 2 : chunk id
// field 3 : obj val

func LambdaPeek(l *redeo.LambdaInstance) {
	for {
		var obj redeo.Response
		// field 0 for cmd
		field0, err := l.R.PeekType()
		if err != nil {
			if err == io.EOF {
				log.Warn("Lambda store disconnected: %s", l.Name)
				l.Closed = true
				return
			} else {
				log.Warn("Failed to peek response type(%s): %v", l.Name, err)
			}
			continue
		}
		t2 := time.Now()

		var cmd string
		switch field0 {
		case resp.TypeError:
			strErr, _ := l.R.ReadError()
			err = errors.New(strErr)
			log.Warn("Error on peek response type: %v", err)
		default:
			cmd, err = l.R.ReadBulkString()
			if err != nil {
				log.Warn("Error on read response type(%s): %v", l.Name, err)
				break
			}

			switch cmd {
			case "get":
			case "set":
				setHandler(l, t2)
				err = errors.New("continue")
			case "data":
				collectDataFromLambda(l)
				err = errors.New("continue")
			default:
				err = errors.New(cmd)
				log.Warn("Unsupport response type(%s): %v", l.Name, err)
			}
		}
		if err != nil {
			continue
		}

		// Get Handler
		// Exhaust all values to keep protocol aligned.
		connId, _ := l.R.ReadBulkString()
		reqId, _ := l.R.ReadBulkString()
		chunkId, _ := l.R.ReadBulkString()
		counter, ok := redeo.ReqMap.Get(reqId)
		if ok == false {
			log.Warn("Request not found(%s): %s", l.Name, reqId)
			// exhaust value field
			l.R.ReadBulk(nil)
			continue
		}

		obj.Cmd = cmd
		obj.Id.ConnId, _ = strconv.Atoi(connId)
		obj.Id.ReqId = reqId
		obj.Id.ChunkId, _ = strconv.ParseInt(chunkId, 10, 64)

		abandon := false
		reqCounter := atomic.AddInt32(&(counter.(*redeo.ClientReqCounter).Counter), 1)
		// Check if chunks are enough? Shortcut response if YES.
		if int(reqCounter) > counter.(*redeo.ClientReqCounter).DataShards {
			abandon = true
			cMap[obj.Id.ConnId] <- &redeo.Chunk{ChunkId: obj.Id.ChunkId, ReqId: obj.Id.ReqId, Cmd: obj.Cmd}
			if err := nanoLog(resp.LogProxy, obj.Cmd, obj.Id.ReqId, obj.Id.ChunkId, t2.UnixNano(), int64(time.Since(t2)), int64(0)); err != nil {
				log.Warn("LogProxy err %v", err)
			}
		}

		// Read value
		t9 := time.Now()
		res, err := l.R.ReadBulk(nil)
		time9 := time.Since(t9)
		if err != nil {
			log.Warn("Failed to read value of response(%s): %v", l.Name, err)
			// Abandon errant data
			res = nil
		}
		// Skip on abandon
		if abandon {
			continue
		}

		cMap[obj.Id.ConnId] <- &redeo.Chunk{ChunkId: obj.Id.ChunkId, ReqId: obj.Id.ReqId, Body: res, Cmd: obj.Cmd}
		time0 := time.Since(t2)
		if err := nanoLog(resp.LogProxy, obj.Cmd, obj.Id.ReqId, obj.Id.ChunkId, t2.UnixNano(), int64(time0), int64(time9)); err != nil {
			log.Warn("LogProxy err %v", err)
		}
	}
}

func setHandler(l *redeo.LambdaInstance, t time.Time) {
	var obj redeo.Response
	connId, _ := l.R.ReadBulkString()
	obj.Id.ConnId, _ = strconv.Atoi(connId)
	obj.Id.ReqId, _ = l.R.ReadBulkString()
	chunkId, _ := l.R.ReadBulkString()
	obj.Id.ChunkId, _ = strconv.ParseInt(chunkId, 10, 64)

	cMap[obj.Id.ConnId] <- &redeo.Chunk{ChunkId: obj.Id.ChunkId, ReqId: obj.Id.ReqId, Body: []byte{1}, Cmd: "set"}
	if err := nanoLog(resp.LogProxy, "set", obj.Id.ReqId, obj.Id.ChunkId, t.UnixNano(), int64(time.Since(t)), int64(0)); err != nil {
		log.Warn("LogProxy err %v", err)
	}
}

// lambda Handler
// lambda handle incoming client request
func lambdaHandler(l *redeo.LambdaInstance) {
	for {
		a := <-l.C /*blocking on lambda facing channel*/
		// check lambda status first
		l.AliveLock.Lock()
		if l.Alive == false {
			log.Info("[Lambda store is not alive, need to activate: %s]", l.Name)
			l.Alive = true
			// trigger lambda
			go lambdaTrigger(l)
		}
		l.AliveLock.Unlock()
		//*
		// req from client
		//*
		// get channel and chunk id
		connId := strconv.Itoa(a.Id.ConnId)
		chunkId := strconv.FormatInt(a.Id.ChunkId, 10)
		// get cmd argument
		cmd := strings.ToLower(a.Cmd)
		switch cmd {
		case "set": /*set or two argument cmd*/
			l.W.MyWriteCmd(a.Cmd, connId, a.Id.ReqId, chunkId, a.Key, a.Body)
			err := l.W.Flush()
			if err != nil {
				log.Error("Flush pipeline error: %v", err)
			}
		case "get": /*get or one argument cmd*/
			l.W.MyWriteCmd(a.Cmd, connId, a.Id.ReqId, "", a.Key)
			err := l.W.Flush()
			if err != nil {
				log.Error("Flush pipeline error: %v", err)
			}
		}
	}
}

func lambdaTrigger(l *redeo.LambdaInstance) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	client := lambda.New(sess, &aws.Config{Region: aws.String("us-east-1")})

	_, err := client.Invoke(&lambda.InvokeInput{FunctionName: aws.String(l.Name)})
	if err != nil {
		log.Error("Error calling LambdaFunction: %v", err)
	}

	log.Info("[Lambda store deactivated: %s]", l.Name)
	l.AliveLock.Lock()
	l.Alive = false
	l.AliveLock.Unlock()
}

func collectDataFromLambda(l *redeo.LambdaInstance) {
	strLen, err := l.R.ReadBulkString()
	len := 0
	if err != nil {
		log.Error("Failed to read length of data from lambda: %v", err)
	} else {
		len, err = strconv.Atoi(strLen)
		if err != nil {
			log.Error("Convert strLen err: %v", err)
		}
	}
	for i := 0; i < len; i++ {
		//op, _ := l.R.ReadBulkString()
		//status, _ := l.R.ReadBulkString()
		//reqId, _ := l.R.ReadBulkString()
		//chunkId, _ := l.R.ReadBulkString()
		//dAppend, _ := l.R.ReadBulkString()
		//dFlush, _ := l.R.ReadBulkString()
		//dTotal, _ := l.R.ReadBulkString()
		dat, _ := l.R.ReadBulkString()
		//fmt.Println("op, reqId, chunkId, status, dTotal, dAppend, dFlush", op, reqId, chunkId, status, dTotal, dAppend, dFlush)
		nanoLog(resp.LogLambda, "data", dat)
	}
	dataCollected.Done()
}
