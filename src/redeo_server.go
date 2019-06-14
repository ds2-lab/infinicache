package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/bsm/redeo/resp"
	"github.com/wangaoone/redeo"
	"net"
	"sync"
)

var (
	clientLis, ok1 = net.Listen("tcp", ":6378")
	lambdaLis, ok2 = net.Listen("tcp", ":6379")
	lambdaStore    *lambdaInstance
	lambdaStoreMap map[lambdaInstance]net.Conn
	instanceLock   sync.Mutex
	cMap           = make(map[int]chan interface{}, 1) // client channel mapping table
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
	if ok1 != nil {
		fmt.Println("listen port failed")
	}
	fmt.Println("start listening client face port 6378")
	fmt.Println("start listening lambda face port 6379")
	srv := redeo.NewServer(nil)
	// initial lambda
	initial()

	// lambda handler
	go lambdaHandler(lambdaStore)

	// Start serving (blocking)
	err := srv.MyServe(clientLis, cMap, lambdaStore.setC)
	if err != nil {
		fmt.Println(err)
	}
}

func initial() {
	instanceLock.Lock()
	if lambdaStore == nil {
		fmt.Println("create new lambda instance")
		lambdaStore = newLambdaInstance("Lambda2SmallJPG")
	}
	instanceLock.Unlock()
	// check lambdaStore alive
	lambdaStore.aliveLock.Lock()
	if lambdaStore.alive == false {
		fmt.Println("Lambda 2 is not alive, need to activate")
		// trigger lambda
		lambdaStore.alive = true
		go lambdaTrigger()
	}
	lambdaStore.aliveLock.Unlock()
	// check lambda connection
	lambdaStore.cnLock.Lock()
	if lambdaStore.cn == nil {
		fmt.Println("start a new conn")
		// start a new server to receive conn from lambda store
		lambdaSrv := redeo.NewServer(nil)
		lambdaStore.cn = lambdaSrv.Accept(lambdaLis)
		// writer and reader
		lambdaStore.w = resp.NewRequestWriter(lambdaStore.cn)
		lambdaStore.r = resp.NewResponseReader(lambdaStore.cn)
	}
	lambdaStore.cnLock.Unlock()
	fmt.Println("lambda store has connected", lambdaStore.cn.RemoteAddr())
}

func lambdaHandler(l *lambdaInstance) {
	//temp := make(chan interface{}, 1)
	for {
		select {
		case a := <-l.setC:
			fmt.Println("in lambda handler", a)
			fmt.Println(a.Cmd)             // cmd
			fmt.Println(a.Argument.Arg(0)) // argument
			if lambdaStore.alive == false {
				fmt.Println("Lambda 2 is not alive, need to activate")
				// trigger lambda
				lambdaStore.alive = true
				go lambdaTrigger()
			}
			// client part
			lambdaStore.w.WriteCmdString("SET", "hello", "world")
			// Flush pipeline
			err := lambdaStore.w.Flush()
			if err != nil {
				fmt.Println("flush pipeline err is ", err)
			}
			// Read response
			// Consume responses
			s, err := lambdaStore.r.ReadInt()
			if err != nil {
				fmt.Println("read err is ", err)
			}
			fmt.Println("received, r is", s)
			cMap[0] <- s

			//go read(temp)
			//case b := <-temp:
			//	fmt.Println("res is ", b)
			//	cMap[0] <- b

		}
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
	fmt.Println("register lambda store", l.name)
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

	fmt.Println("Lambda Deactivate")
	lambdaStore.aliveLock.Lock()
	lambdaStore.alive = false
	lambdaStore.aliveLock.Unlock()
}
