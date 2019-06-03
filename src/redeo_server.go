package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/bsm/redeo/resp"
	"github.com/wangaoone/redeo"
	"net"
)

var (
	getChan1     = make(chan resp.CommandArgument, 1)
	getChan2     = make(chan []byte, 1)
	setChan1     = make(chan setObject, 1)
	setChan2     = make(chan int64, 1)
	clientLis, _ = net.Listen("tcp", ":6378")
	lambdaLis, _ = net.Listen("tcp", ":6379")
	lambdaStore  *lambdaInstance
)

type setObject struct {
	key   resp.CommandArgument
	value resp.CommandArgument
}

type lambdaInstance struct {
	name  string
	alive bool
	cn    net.Conn
}

func newLambdaInstance(name string) *lambdaInstance {
	return &lambdaInstance{
		name:  name,
		alive: false,
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
		getChan1 <- c.Arg(0)
		fmt.Println("sending to lambda...")
		// trigger trigger lambda store
		go getLambda()
		// get return from channel
		return <-getChan2
	}))

	srv.Handle("set", redeo.WrapperFunc(func(c *resp.Command) interface{} {
		if c.ArgN() != 2 {
			return redeo.ErrWrongNumberOfArgs(c.Name)
		}
		fmt.Println("command from client", c.Arg(0))
		setChan1 <- setObject{c.Arg(0), c.Arg(1)}
		fmt.Println("sending to lambda...")
		// trigger trigger lambda store
		go setLambda()
		// get return from channel
		return <-setChan2
	}))

	//defer lis.Close()

	// Start serving (blocking)
	err := srv.Serve(clientLis)
	if err != nil {
		fmt.Println(err)
	}

}

func setLambda() {
	fmt.Println("===== set to lambda function start =====")
	setObj := <-setChan1
	//fmt.Println("ready send to lambda storage", " the obj key is", setObj.key.String(), "value is ", setObj.value.String())
	if lambdaStore == nil {
		fmt.Println("create new lambda instance")
		lambdaStore = newLambdaInstance("Lambda2SmallJPG")
	}
	if lambdaStore.alive == false {
		fmt.Println("Lambda 2 is not alive, need to activate")
		// trigger lambda
		go lambdaTrigger(lambdaStore)
	}

	if lambdaStore.cn == nil {
		fmt.Println("start a new conn")
		// start a new server to receive conn from lambda store
		srv := redeo.NewServer(nil)
		lambdaStore.cn = srv.Accept(lambdaLis)
	}
	fmt.Println("lambda store has connected", lambdaStore.cn.RemoteAddr())

	// client part
	// writer and reader
	w := resp.NewRequestWriter(lambdaStore.cn)
	r := resp.NewResponseReader(lambdaStore.cn)

	w.WriteCmdString("SET", setObj.key.String(), setObj.value.String())
	//fmt.Println("write comd", w)

	// Flush pipeline
	err := w.Flush()
	if err != nil {
		fmt.Println("flush err is ", err)
	}

	// Read response
	// Consume responses
	s, err := r.ReadInt()
	if err != nil {
		fmt.Println("ReadBulk err is", err)
	}
	fmt.Println("received")
	setChan2 <- s
	fmt.Println("===== set to lambda function over =====")
}

func getLambda() {
	fmt.Println("===== put to lambda function start =====")
	key := <-getChan1
	//fmt.Println("ready send to lambda storage, the obj name is", string(key), "cmd", key)
	fmt.Println("start listening lambda face port 6379")
	if lambdaStore == nil {
		fmt.Println("create new lambda instance")
		lambdaStore = newLambdaInstance("Lambda2SmallJPG")
	}
	if lambdaStore.alive == false {
		fmt.Println("Lambda 2 is not alive, need to activate")
		// trigger lambda
		go lambdaTrigger(lambdaStore)
	}

	if lambdaStore.cn == nil {
		fmt.Println("start a new conn")
		// start a new server to receive conn from lambda store
		srv := redeo.NewServer(nil)
		lambdaStore.cn = srv.Accept(lambdaLis)
	}
	fmt.Println("lambda store has connected", lambdaStore.cn.RemoteAddr())

	// client part
	// writer and reader
	w := resp.NewRequestWriter(lambdaStore.cn)
	r := resp.NewResponseReader(lambdaStore.cn)

	w.WriteCmdString("get", string(key))
	//fmt.Println("write comd", w)

	// Flush pipeline
	err := w.Flush()
	if err != nil {
		fmt.Println("flush err is ", err)
	}

	// Read response
	buf := make([]byte, 0)
	// Consume responses
	s, err := r.ReadBulk(buf)
	if err != nil {
		fmt.Println("ReadBulk err is", err)
	}
	fmt.Println("received")
	getChan2 <- s
	fmt.Println("===== put to lambda function over =====")
}

func lambdaTrigger(l *lambdaInstance) {
	//fmt.Println("triggering lambda")
	l.alive = true

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	client := lambda.New(sess, &aws.Config{Region: aws.String("us-east-1")})

	_, err := client.Invoke(&lambda.InvokeInput{FunctionName: aws.String(l.name)})
	if err != nil {
		fmt.Println("Error calling LambdaFunction")
	}

	fmt.Println("lamdba deactive")
	l.alive = false
}
