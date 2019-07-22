package main

import (
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/pkg/profile"
	"github.com/wangaoone/ecRedis"
	"github.com/wangaoone/redeo"
	"github.com/wangaoone/redeo/resp"
	"net"
	"strconv"
	"strings"
	"time"
)

var (
	replica = flag.Bool("replica", true, "Enable lambda replica deployment")
	isPrint = flag.Bool("isPrint", false, "Enable log printing")
)

var (
	clientLis net.Listener
	lambdaLis net.Listener
	cMap      = make(map[int]chan interface{}) // client channel mapping table
)

func main() {
	// CPU profiling by default
	defer profile.Start().Stop()

	flag.Parse()
	fmt.Println("======================================")
	fmt.Println("replica:", *replica, "||", "isPrint:", *isPrint)
	fmt.Println("======================================")
	clientLis, _ = net.Listen("tcp", ":6378")
	lambdaLis, _ = net.Listen("tcp", ":6379")
	fmt.Println("start listening client face port 6378")
	fmt.Println("start listening lambda face port 6379")
	srv := redeo.NewServer(nil)
	lambdaSrv := redeo.NewServer(nil)

	// initial lambda store group
	group := initial(lambdaSrv)

	// Start serving (blocking)
	err := srv.MyServe(clientLis, cMap, group)
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
			myPrint("start a new conn")
			node.Cn = lambdaSrv.Accept(lambdaLis)
			myPrint("lambda store has connected", node.Cn.RemoteAddr())
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
			myPrint("start a new conn")
			node.Cn = lambdaSrv.Accept(lambdaLis)
			myPrint("lambda store has connected", node.Cn.RemoteAddr())
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
		C:     make(chan redeo.Req, 1024*1024),
	}
}

// blocking on lambda peek Type
// lambda handle incoming lambda store response
//
// field 0 : client id
// field 1 : chunk id
// field 2 : obj val

func LambdaPeek(l *redeo.LambdaInstance) {
	for {
		var obj redeo.Response
		// field 0 for client id
		// bulkString
		t2 := time.Now()
		field1, err := l.R.PeekType()
		if err != nil {
			fmt.Println("field1 err", err)
			return
		}
		time2 := time.Since(t2)
		t3 := time.Now()
		switch field1 {
		case resp.TypeInt:
			clientId, _ := l.R.ReadInt()
			obj.Id.ClientId = int(clientId)
		case resp.TypeError:
			err, _ := l.R.ReadError()
			fmt.Println("peek type err1 is", err)
		default:
			panic("unexpected response type")
		}
		time3 := time.Since(t3)
		//
		// field 1 for chunk id
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
		case resp.TypeInt:
			chunkId, _ := l.R.ReadInt()
			obj.Id.ChunkId = int(chunkId)
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
			//index := redeo.Index{ClientId: obj.Id.ClientId, ReqId: obj.Id.ReqId}
			res, err := l.R.ReadBulk(nil)
			if err != nil {
				fmt.Println("response err is ", err)
			}
			chunk := redeo.Chunk{Id: obj.Id.ChunkId, Body: res}
			cMap[obj.Id.ClientId] <- chunk
		case resp.TypeInt:
			//index := redeo.Index{ClientId: obj.Id.ClientId, ReqId: obj.Id.ReqId}
			_, err := l.R.ReadInt()
			if err != nil {
				fmt.Println("response err is ", err)
			}
			chunk := redeo.Chunk{Id: obj.Id.ChunkId, Body: []byte{1}}
			cMap[obj.Id.ClientId] <- chunk
		case resp.TypeError:
			err, _ := l.R.ReadError()
			fmt.Println("peek type err4 is", err)
			return
		default:
			panic("unexpected response type")
		}
		time9 := time.Since(t9)
		myPrint(obj.Id.ClientId, obj.Id.ChunkId,
			"Sever PeekType clientId time is", time2,
			"Sever read field0 clientId time is", time3,
			"Sever PeekType chunkId time is", time6,
			"Sever read field1 chunkId time is", time7,
			"Sever PeekType objBody time is", time8,
			"Sever read field2 chunkBody time is", time9)
	}
}

// lambda Handler
// lambda handle incoming client request
func lambdaHandler(l *redeo.LambdaInstance) {
	fmt.Println("conn is", l.Cn)
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
		clientId := strconv.Itoa(a.Id.ClientId)
		//reqId := strconv.Itoa(a.Id.ReqId)
		chunkId := strconv.Itoa(a.Id.ChunkId)
		//fmt.Println("client, chunk, reqId", clientId, chunkId, reqId)
		// get cmd argument
		cmd := strings.ToLower(a.Cmd)
		switch cmd {
		case "set": /*set or two argument cmd*/
			l.W.MyWriteCmd(a.Cmd, clientId, "", chunkId, a.Key, a.Val)
			err := l.W.Flush()
			if err != nil {
				fmt.Println("flush pipeline err is ", err)
			}
		case "get": /*get or one argument cmd*/
			l.W.MyWriteCmd(a.Cmd, clientId, "", "", a.Key)
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
