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
	//getChan1     = make(chan resp.CommandArgument, 1)
	//getChan2     = make(chan string, 1)
	clientLis, _ = net.Listen("tcp", ":6378")
	lambdaLis, _ = net.Listen("tcp", ":6379")
	lambdaStore  *lambdaInstance
	instanceLock sync.Mutex
	aliveLock    sync.Mutex
	cnLock       sync.Mutex
	tempLock     sync.Mutex
	tempLock2    sync.Mutex
)

type setObject struct {
	key   resp.CommandArgument
	value resp.CommandArgument
}

type lambdaInstance struct {
	name  string
	alive bool
	cn    net.Conn
	w     *resp.RequestWriter
	r     resp.ResponseReader
	c     chan interface{}
}

func newLambdaInstance(name string) *lambdaInstance {
	return &lambdaInstance{
		name:  name,
		alive: false,
		c:     make(chan interface{}, 1),
	}
}

func main() {
	fmt.Println("start listening client face port 6378")
	fmt.Println("start listening lambda face port 6379")
	srv := redeo.NewServer(nil)

	// Define handlers
	srv.Handle("get", redeo.WrapperFunc(func(c *resp.Command) interface{} {
		if c.ArgN() != 1 {
			return redeo.ErrWrongNumberOfArgs(c.Name)
		}
		//getChan1 <- c.Arg(0)
		// trigger trigger lambda store
		key := c.Arg(0)
		obj := getLambda(key)
		// get return from channel
		return obj
	}))

	srv.Handle("set", redeo.WrapperFunc(func(c *resp.Command) interface{} {
		if c.ArgN() != 2 {
			return redeo.ErrWrongNumberOfArgs(c.Name)
		}
		//setChan1 <- setObject{c.Arg(0), c.Arg(1)}
		obj := setObject{c.Arg(0), c.Arg(1)}
		myResp := setLambda(obj)
		// get return from channel
		//return <-setChan2
		//return <-lambdaStore.c
		//return respInt
		//for range lambdaStore.c {
		//	fmt.Println("lambda channel is ", <-lambdaStore.c)
		//}
		//fmt.Println("lambda channel is ", <-lambdaStore.c)

		return myResp
	}))

	//defer lis.Close()

	// Start serving (blocking)
	err := srv.Serve(clientLis)
	if err != nil {
		fmt.Println(err)
	}
}

func setLambda(object setObject) int64 {
	fmt.Println("===== set to lambda function start =====")
	//setObj := <-setChan1
	// check lambdaStore instance
	instanceLock.Lock()
	if lambdaStore == nil {
		fmt.Println("create new lambda instance")
		lambdaStore = newLambdaInstance("Lambda2SmallJPG")
	}
	instanceLock.Unlock()
	// check lambdaStore alive
	aliveLock.Lock()
	if lambdaStore.alive == false {
		fmt.Println("Lambda 2 is not alive, need to activate")
		// trigger lambda
		lambdaStore.alive = true

		go lambdaTrigger()
	}
	aliveLock.Unlock()
	// check lambda connection
	cnLock.Lock()
	if lambdaStore.cn == nil {
		fmt.Println("start a new conn")
		// start a new server to receive conn from lambda store
		srv := redeo.NewServer(nil)
		lambdaStore.cn = srv.Accept(lambdaLis)
		// writer and reader
		lambdaStore.w = resp.NewRequestWriter(lambdaStore.cn)
		lambdaStore.r = resp.NewResponseReader(lambdaStore.cn)
	}
	cnLock.Unlock()
	fmt.Println("lambda store has connected", lambdaStore.cn.RemoteAddr())

	// client part
	tempLock2.Lock()
	lambdaStore.w.WriteCmdString("SET", object.key.String(), object.value.String())
	// Flush pipeline
	err := lambdaStore.w.Flush()
	if err != nil {
		fmt.Println("flush pipeline err is ", err)
	}
	tempLock2.Unlock()
	// Read response
	// Consume responses
	fmt.Println("lock read")
	tempLock.Lock()
	s, err := lambdaStore.r.ReadInt()
	if err != nil {
		fmt.Println("read err is ", err)
	}
	fmt.Println("received, r is", s)
	tempLock.Unlock()
	fmt.Println("Unlock read")

	fmt.Println("===== set to lambda function over =====")
	return s
}

func getLambda(key resp.CommandArgument) string {
	fmt.Println("===== get to lambda function start =====")
	//key := <-getChan1
	instanceLock.Lock()
	if lambdaStore == nil {
		fmt.Println("create new lambda instance")
		lambdaStore = newLambdaInstance("Lambda2SmallJPG")
	}
	instanceLock.Unlock()
	aliveLock.Lock()
	if lambdaStore.alive == false {
		fmt.Println("Lambda 2 is not alive, need to activate")
		// trigger lambda
		lambdaStore.alive = true
		go lambdaTrigger()
	}
	aliveLock.Unlock()
	cnLock.Lock()
	if lambdaStore.cn == nil {
		fmt.Println("start a new conn")
		// start a new server to receive conn from lambda store
		srv := redeo.NewServer(nil)
		lambdaStore.cn = srv.Accept(lambdaLis)
		// writer and reader
		lambdaStore.w = resp.NewRequestWriter(lambdaStore.cn)
		lambdaStore.r = resp.NewResponseReader(lambdaStore.cn)
	}
	cnLock.Unlock()
	fmt.Println("lambda store has connected", lambdaStore.cn.RemoteAddr())

	// client part
	tempLock2.Lock()
	lambdaStore.w.WriteCmdString("get", key.String())
	// Flush pipeline
	err := lambdaStore.w.Flush()
	if err != nil {
		fmt.Println("flush err is ", err)
	}
	tempLock2.Unlock()

	// Read response
	//buf := make([]byte, 0)
	// Consume responses
	//s, err := r.ReadBulk(buf)
	tempLock.Lock()
	s, err := lambdaStore.r.ReadBulkString()
	if err != nil {
		fmt.Println("ReadBulk err is", err)
	}
	fmt.Println("get it!")
	//getChan2 <- s
	tempLock.Unlock()
	fmt.Println("===== get to lambda function over =====")
	return s
}

func lambdaTrigger() {

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	client := lambda.New(sess, &aws.Config{Region: aws.String("us-east-1")})

	_, err := client.Invoke(&lambda.InvokeInput{FunctionName: aws.String(lambdaStore.name)})
	if err != nil {
		fmt.Println("Error calling LambdaFunction")
	}

	fmt.Println("lamdba deactive")
	aliveLock.Lock()
	lambdaStore.alive = false
	aliveLock.Unlock()
}
