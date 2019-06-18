package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/bsm/redeo/resp"
	"github.com/kelindar/binary"
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
	isPrint        = false
	temp           = make(chan Response, 1)
)

type Response struct {
	Id   string
	Body string
}

type lambdaInstance struct {
	name      string
	alive     bool
	cn        net.Conn
	w         *resp.RequestWriter
	r         resp.ResponseReader
	c         chan redeo.Req
	aliveLock sync.Mutex
	cnLock    sync.Mutex
	tempLock  sync.Mutex
}

func newLambdaInstance(name string) *lambdaInstance {
	return &lambdaInstance{
		name:  name,
		alive: false,
		c:     make(chan redeo.Req, 1),
	}
}

func main() {
	fmt.Println("start listening client face port 6378")
	fmt.Println("start listening lambda face port 6379")
	srv := redeo.NewServer(nil)
	// initial lambda
	initial()

	// lambda handler
	go lambdaHandler(lambdaStore)
	go myPeek(lambdaStore)

	// Start serving (blocking)
	err := srv.MyServe(clientLis, cMap, lambdaStore.c)
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
		//obj := receiveObj{"", -1}
		t, err := l.r.PeekType()
		if err != nil {
			return
		}

		switch t {
		case resp.TypeBulk:
			n, _ := l.r.ReadBulk(nil)
			var obj Response
			err := binary.Unmarshal(n, &obj)
			if err != nil {
				fmt.Println(err)
			}
			myPrint("res is ", obj)
			temp <- obj
		case resp.TypeError:
			err, _ := l.r.ReadError()
			fmt.Println(err)
		default:
			panic("unexpected response type")
		}
	}
}
func lambdaHandler(l *lambdaInstance) {
	for {
		select {
		case a := <-l.c:
			myPrint("req from client is ", a.Cmd, a.Argument)
			// client part
			// get channel id
			cid := strconv.Itoa(a.Cid)
			argsCount := len(a.Argument.Args)

			lambdaStore.aliveLock.Lock()
			if lambdaStore.alive == false {
				myPrint("Lambda 2 is not alive, need to activate")
				// trigger lambda
				lambdaStore.alive = true
				go lambdaTrigger()
			}
			lambdaStore.aliveLock.Unlock()
			switch argsCount {
			case 1:
				lambdaStore.w.WriteCmdString(a.Cmd, a.Argument.Arg(0).String(), cid)
				// Flush pipeline
				err := lambdaStore.w.Flush()
				if err != nil {
					fmt.Println("flush pipeline err is ", err)
				}
			case 2:
				myPrint("obj length is ", len(a.Argument.Arg(1)))
				lambdaStore.w.WriteCmdString(a.Cmd, a.Argument.Arg(0).String(), a.Argument.Arg(1).String(), cid)
				// Flush pipeline
				err := lambdaStore.w.Flush()
				if err != nil {
					fmt.Println("flush pipeline err is ", err)
				}
				myPrint("write complete")
			}
		case obj := <-temp:
			// parse client channel id
			id, _ := strconv.Atoi(obj.Id)
			// send response body to client channel
			cMap[id] <- obj.Body
		}
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
