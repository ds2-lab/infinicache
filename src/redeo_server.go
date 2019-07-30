package main

import (
	"flag"
	"fmt"
	"github.com/ScottMansfield/nanolog"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/wangaoone/ecRedis"
	"github.com/wangaoone/redeo"
	"github.com/wangaoone/redeo/resp"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var (
	replica = flag.Bool("replica", true, "Enable lambda replica deployment")
	isPrint = flag.Bool("isPrint", false, "Enable log printing")
	prefix  = flag.String("prefix", "log", "log file prefix")
)

var (
	clientLis net.Listener
	lambdaLis net.Listener
	cMap      = make(map[int]chan interface{}) // client channel mapping table
	filePath  = "/var/run/pidLog.txt"
)

func logCreate() {
	// get local time
	location, _ := time.LoadLocation("EST")
	// Set up nanoLog writer
	nanoLogout, err := os.Create(time.Now().In(location).String() + *prefix + "_proxy.clog")
	if err != nil {
		panic(err)
	}
	err = nanolog.SetWriter(nanoLogout)
	if err != nil {
		panic(err)
	}
}

func main() {
	flag.Parse()
	// CPU profiling by default
	//defer profile.Start().Stop()
	// init log
	logCreate()
	// Log goroutine
	//defer t.Stop()
	go func() {
		t := time.NewTicker(10 * time.Second)
		for {
			select {
			case <-t.C:
				if err := nanolog.Flush(); err != nil {
					fmt.Println("log flush err")
				}
			}
		}
	}()
	fmt.Println("======================================")
	fmt.Println("replica:", *replica, "||", "isPrint:", *isPrint)
	fmt.Println("======================================")
	clientLis, _ = net.Listen("tcp", ":6378")
	lambdaLis, _ = net.Listen("tcp", ":6379")
	fmt.Println("start listening client face port :6378，lambda face port :6379")
	// initial proxy and lambda server
	srv := redeo.NewServer(nil)
	lambdaSrv := redeo.NewServer(nil)

	// initial lambda store group
	group := initial(lambdaSrv)

	ioutil.WriteFile(filePath, []byte(fmt.Sprintf("%d", os.Getpid())), 0660)
	// Start serving (blocking)
	err := srv.MyServe(clientLis, cMap, group, filePath)
	if err != nil {
		fmt.Println(err)
	}
}

// initial lambda group
func initial(lambdaSrv *redeo.Server) redeo.Group {
	group := redeo.Group{Arr: make([]redeo.LambdaInstance, ecRedis.MaxLambdaStores), MemCounter: 0}
	if *replica == true {
		for i := range group.Arr {
			node := newLambdaInstance("Lambda2SmallJPG")
			myPrint("No.", i, "replication lambda store has registered")
			// register lambda instance to group
			group.Arr[i] = *node
			node.Alive = true
			go lambdaTrigger(node)
			// start a new server to receive conn from lambda store
			node.Cn = lambdaSrv.Accept(lambdaLis)
			myPrint("start a new conn, lambda store has connected", node.Cn.RemoteAddr())
			// wrap writer and reader
			node.W = resp.NewRequestWriter(node.Cn)
			node.R = resp.NewResponseReader(node.Cn)
			// lambda handler
			go lambdaHandler(node)
			// lambda facing peeking response type
			go LambdaPeek(node)
			myPrint(node.Alive)
		}
	} else {
		for i := range group.Arr {
			node := newLambdaInstance("Node" + strconv.Itoa(i))
			myPrint(node.Name, "lambda store has registered")
			// register lambda instance to group
			group.Arr[i] = *node
			node.Alive = true
			go lambdaTrigger(node)
			// start a new server to receive conn from lambda store
			node.Cn = lambdaSrv.Accept(lambdaLis)
			myPrint("start a new conn, lambda store has connected", node.Cn.RemoteAddr())
			// wrap writer and reader
			node.W = resp.NewRequestWriter(node.Cn)
			node.R = resp.NewResponseReader(node.Cn)
			// lambda handler
			go lambdaHandler(node)
			// lambda facing peeking response type
			go LambdaPeek(node)
			myPrint(node.Alive)
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
		//
		// field 0 for conn id
		// bulkString
		t2 := time.Now()
		field0, err := l.R.PeekType()
		if err != nil {
			fmt.Println("field1 err", err)
			return
		}
		time2 := time.Since(t2)
		t3 := time.Now()
		switch field0 {
		case resp.TypeBulk:
			connId, _ := l.R.ReadBulkString()
			obj.Id.ConnId, _ = strconv.Atoi(connId)
			fmt.Println("conn id", obj.Id.ConnId)
		case resp.TypeError:
			err, _ := l.R.ReadError()
			fmt.Println("peek type err1 is", err)
		default:
			panic("unexpected response type")
		}
		time3 := time.Since(t3)
		//
		// field 1 for req id
		// bulkString
		field1, err := l.R.PeekType()
		if err != nil {
			fmt.Println("field1 err", err)
			return
		}
		// read field 1
		//var ReqCounter int32
		abandon := false
		switch field1 {
		case resp.TypeBulk:
			reqId, _ := l.R.ReadBulkString()
			counter, ok := redeo.ReqMap.Get(reqId)
			if ok == false {
				fmt.Println("No reqId found")
			}
			// reqCounter++
			reqCounter := atomic.AddInt32(&(counter.(*redeo.ClientReqCounter).Counter), 1)
			//myPrint("cmd is", counter.(*redeo.ClientReqCounter).Cmd, "atomic counter is", int(reqCounter), "dataShards int", counter.(*redeo.ClientReqCounter).DataShards)
			if int(reqCounter) > counter.(*redeo.ClientReqCounter).DataShards && counter.(*redeo.ClientReqCounter).Cmd == "get" {
				abandon = true
			}
		case resp.TypeError:
			err, _ := l.R.ReadError()
			fmt.Println("peek type err1 is", err)
			panic("peek type err")
		default:
			panic("unexpected response type")
		}
		//
		// field 2 for chunk id
		// Int
		t6 := time.Now()
		field3, err := l.R.PeekType()
		if err != nil {
			fmt.Println("field3 err", err)
			return
		}
		time6 := time.Since(t6)
		t7 := time.Now()
		switch field3 {
		case resp.TypeBulk:
			chunkId, _ := l.R.ReadBulkString()
			obj.Id.ChunkId, _ = strconv.Atoi(chunkId)
		case resp.TypeError:
			err, _ := l.R.ReadError()
			fmt.Println("peek type err3 is", err)
		default:
			panic("unexpected response type")
		}
		time7 := time.Since(t7)
		//
		// field 2 for obj body
		// bulkString
		t8 := time.Now()
		field4, err := l.R.PeekType()
		if err != nil {
			fmt.Println("field4 err", err)
			return
		}
		time8 := time.Since(t8)
		t9 := time.Now()
		switch field4 {
		case resp.TypeBulk:
			res, err := l.R.ReadBulk(nil)
			if err != nil {
				fmt.Println("response err is ", err)
			}
			if !abandon {
				myPrint("Abandon is ", abandon)
				cMap[obj.Id.ConnId] <- &redeo.Chunk{Id: obj.Id.ChunkId, Body: res}
			} else {
				cMap[obj.Id.ConnId] <- &redeo.Chunk{Id: -1}
			}
		case resp.TypeInt:
			_, err := l.R.ReadInt()
			if err != nil {
				fmt.Println("response err is ", err)
			}
			cMap[obj.Id.ConnId] <- &redeo.Chunk{Id: obj.Id.ChunkId, Body: []byte{1}}
		case resp.TypeError:
			err, _ := l.R.ReadError()
			fmt.Println("peek type err4 is", err)
			return
		default:
			panic("unexpected response type")
		}
		time9 := time.Since(t9)
		//myPrint(obj.Id.ConnId, obj.Id.ChunkId,
		//	"Sever PeekType clientId time is", time2,
		//	"Sever read field0 clientId time is", time3,
		//	"Sever PeekType chunkId time is", time6,
		//	"Sever read field1 chunkId time is", time7,
		//	"Sever PeekType objBody time is", time8,
		//	"Sever read field2 chunkBody time is", time9)
		if err := nanolog.Log(resp.LogProxy, obj.Id.ConnId, obj.Id.ChunkId,
			time2.String(), time3.String(), time6.String(), time7.String(), time8.String(), time9.String()); err != nil {
			fmt.Println("LogProxy err ", err)
		}
	}
}

// lambda Handler
// lambda handle incoming client request
func lambdaHandler(l *redeo.LambdaInstance) {
	myPrint("conn is", l.Cn)
	for {
		a := <-l.C /*blocking on lambda facing channel*/
		// check lambda status first
		l.AliveLock.Lock()
		if l.Alive == false {
			myPrint("Lambda store is not alive, need to activate")
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
		chunkId := strconv.Itoa(a.Id.ChunkId)
		// get cmd argument
		cmd := strings.ToLower(a.Cmd)
		switch cmd {
		case "set": /*set or two argument cmd*/
			l.W.MyWriteCmd(a.Cmd, connId, a.Id.ReqId, chunkId, a.Key, a.Body)
			err := l.W.Flush()
			if err != nil {
				fmt.Println("flush pipeline err is ", err)
			}
		case "get": /*get or one argument cmd*/
			l.W.MyWriteCmd(a.Cmd, connId, a.Id.ReqId, "", a.Key)
			err := l.W.Flush()
			if err != nil {
				fmt.Println("flush pipeline err is ", err)
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
		fmt.Println("Error calling LambdaFunction", err)
	}

	myPrint("Lambda Deactivate")
	l.AliveLock.Lock()
	l.Alive = false
	l.AliveLock.Unlock()
}

func myPrint(a ...interface{}) {
	if *isPrint {
		fmt.Println(a)
	}
}
