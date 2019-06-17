package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/bsm/redeo/resp"
	"github.com/wangaoone/redeo"
	"net"
	"strconv"
	"sync"
)

var (
	clientLis, _   = net.Listen("tcp", ":6378")
	lambdaLis, _   = net.Listen("tcp", ":6379")
	lambdaStore    *lambdaInstance
	lambdaStoreMap map[lambdaInstance]net.Conn
	instanceLock   sync.Mutex
	cMap           = make(map[int]chan interface{}, 1) // client channel mapping table
	isPrint        = true
	temp           = make(chan int, 1)
)

type lambdaInstance struct {
	name      string
	alive     bool
	cn        net.Conn
	w         *resp.RequestWriter
	r         resp.ResponseReader
	setC      chan redeo.Req
	getC      chan string
	aliveLock sync.Mutex
	cnLock    sync.Mutex
	tempLock  sync.Mutex
}

func newLambdaInstance(name string) *lambdaInstance {
	return &lambdaInstance{
		name:  name,
		alive: false,
		setC:  make(chan redeo.Req, 1),
	}
}

func main() {
	myPrint("start listening client face port 6378")
	myPrint("start listening lambda face port 6379")
	srv := redeo.NewServer(nil)
	// initial lambda
	initial()

	// lambda handler
	go lambdaHandler(lambdaStore)
	go myPeek(lambdaStore)

	// Start serving (blocking)
	err := srv.MyServe(clientLis, cMap, lambdaStore.setC)
	if err != nil {
		fmt.Println(err)
	}
}

func initial() {
	instanceLock.Lock()
	if lambdaStore == nil {
		myPrint("create new lambda instance")
		lambdaStore = newLambdaInstance("Lambda2SmallJPG")
	}
	instanceLock.Unlock()
	// check lambdaStore alive
	lambdaStore.aliveLock.Lock()
	if lambdaStore.alive == false {
		myPrint("Lambda 2 is not alive, need to activate")
		// trigger lambda
		lambdaStore.alive = true
		go lambdaTrigger()
	}
	lambdaStore.aliveLock.Unlock()
	// check lambda connection
	lambdaStore.cnLock.Lock()
	if lambdaStore.cn == nil {
		myPrint("start a new conn")
		// start a new server to receive conn from lambda store
		lambdaSrv := redeo.NewServer(nil)
		lambdaStore.cn = lambdaSrv.Accept(lambdaLis)
		// writer and reader
		lambdaStore.w = resp.NewRequestWriter(lambdaStore.cn)
		lambdaStore.r = resp.NewResponseReader(lambdaStore.cn)
	}
	lambdaStore.cnLock.Unlock()
	myPrint("lambda store has connected", lambdaStore.cn.RemoteAddr())
}

func myPeek(l *lambdaInstance) {
	for {
		t, err := l.r.PeekType()
		if err != nil {
			return
		}

		switch t {
		case resp.TypeBulk:
			n, _ := l.r.ReadBulkString()
			id, _ := strconv.Atoi(n)
			temp <- id
		default:
			panic("unexpected response type")
		}
	}
}
func lambdaHandler(l *lambdaInstance) {
	//temp := make(chan interface{}, 1)
	for {
		select {
		case a := <-l.setC:
			if lambdaStore.alive == false {
				myPrint("Lambda 2 is not alive, need to activate")
				// trigger lambda
				lambdaStore.alive = true
				go lambdaTrigger()
			}
			myPrint("req from client is ", a.Cmd, a.Argument)
			// client part
			// get channel
			cid := strconv.Itoa(a.Cid)
			argsCount := len(a.Argument.Args)

			switch argsCount {
			case 1:
				lambdaStore.w.WriteCmdString(a.Cmd, a.Argument.Arg(0).String(), cid)
				break
			case 2:
				fmt.Println("obj length is ", len(a.Argument.Arg(1)))
				lambdaStore.w.WriteCmdString(a.Cmd, a.Argument.Arg(0).String(), a.Argument.Arg(1).String(), cid)
				break
			default:
				myPrint("wrong cmd")
				break
			}
			// Flush pipeline
			err := lambdaStore.w.Flush()
			if err != nil {
				fmt.Println("flush pipeline err is ", err)
			}
		case channelId := <-temp:
			myPrint("channel id is ", channelId)
			cMap[channelId] <- 1
		}
		// Read response
		// Consume responses
		//	for {
		//		s, err := lambdaStore.r.ReadBulkString()
		//		if err != nil {
		//			fmt.Println("read err is ", err)
		//		}
		//		myPrint("received, r is", s)
		//	}
		//	//channelId, _ := strconv.Atoi(s)
		//	//cMap[channelId] <- 1
		//
		//	//go read(temp)
		//	//case b := <-temp:
		//	//	fmt.Println("res is ", b)
		//	//	cMap[0] <- b
		//
		//}
	}
}

func read(c chan interface{}) {
	for {
		s, err := lambdaStore.r.ReadInt()
		if err != nil {
			fmt.Println("read err is ", err)
		}
		c <- s
	}
}

func register(l *lambdaInstance) {
	lambdaStoreMap[*l] = l.cn
	myPrint("register lambda store", l.name)
}

func lambdaTrigger() {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	client := lambda.New(sess, &aws.Config{Region: aws.String("us-east-1")})

	_, err := client.Invoke(&lambda.InvokeInput{FunctionName: aws.String(lambdaStore.name)})
	if err != nil {
		fmt.Println("Error calling LambdaFunction", err)
	}

	myPrint("Lambda Deactivate")
	lambdaStore.aliveLock.Lock()
	lambdaStore.alive = false
	lambdaStore.aliveLock.Unlock()
}

func myPrint(a ...interface{}) {
	if isPrint == true {
		fmt.Println(a)
	}
}
