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
	clientLis, _   = net.Listen("tcp", ":6378")
	lambdaLis, _   = net.Listen("tcp", ":6379")
	lambdaStore    *lambdaInstance
	lambdaStoreMap map[lambdaInstance]string
	instanceLock   sync.Mutex
)

type setObject struct {
	key   resp.CommandArgument
	value resp.CommandArgument
}

type lambdaInstance struct {
	name      string
	alive     bool
	cn        net.Conn
	w         *resp.RequestWriter
	r         resp.ResponseReader
	c         chan interface{}
	aliveLock sync.Mutex
	cnLock    sync.Mutex
	tempLock  sync.Mutex
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
		if c.ArgN() != 3 {
			fmt.Println("incorrect number of argument")
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
		srv := redeo.NewServer(nil)
		lambdaStore.cn = srv.Accept(lambdaLis)
		// writer and reader
		lambdaStore.w = resp.NewRequestWriter(lambdaStore.cn)
		lambdaStore.r = resp.NewResponseReader(lambdaStore.cn)
	}
	lambdaStore.cnLock.Unlock()
	fmt.Println("lambda store has connected", lambdaStore.cn.RemoteAddr())

	// client part
	lambdaStore.tempLock.Lock()
	fmt.Println("lock client")
	lambdaStore.w.WriteCmdString("SET", object.key.String(), object.value.String())
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
	lambdaStore.tempLock.Unlock()
	fmt.Println("Unlock client")

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
	lambdaStore.aliveLock.Lock()
	if lambdaStore.alive == false {
		fmt.Println("Lambda 2 is not alive, need to activate")
		// trigger lambda
		lambdaStore.alive = true
		go lambdaTrigger()
	}
	lambdaStore.aliveLock.Unlock()
	lambdaStore.cnLock.Lock()
	if lambdaStore.cn == nil {
		fmt.Println("start a new conn")
		// start a new server to receive conn from lambda store
		srv := redeo.NewServer(nil)
		lambdaStore.cn = srv.Accept(lambdaLis)
		// writer and reader
		lambdaStore.w = resp.NewRequestWriter(lambdaStore.cn)
		lambdaStore.r = resp.NewResponseReader(lambdaStore.cn)
	}
	lambdaStore.cnLock.Unlock()
	fmt.Println("Lambda store has connected", lambdaStore.name, lambdaStore.cn.RemoteAddr())

	// client part
	lambdaStore.tempLock.Lock()
	lambdaStore.w.WriteCmdString("get", key.String())
	// Flush pipeline
	err := lambdaStore.w.Flush()
	if err != nil {
		fmt.Println("Flush err is ", err)
	}
	// Read response
	//buf := make([]byte, 0)
	// Consume responses
	//s, err := r.ReadBulk(buf)
	s, err := lambdaStore.r.ReadBulkString()
	if err != nil {
		fmt.Println("ReadBulk err is", err)
	}
	fmt.Println("get it!")
	//getChan2 <- s
	lambdaStore.tempLock.Unlock()
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

	fmt.Println("Lambda Deactivate")
	lambdaStore.aliveLock.Lock()
	lambdaStore.alive = false
	lambdaStore.aliveLock.Unlock()
}
