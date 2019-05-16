package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/kelindar/binary"
	"io"
	"net"
	"time"
)

type lambdaInstance struct {
	name  string
	conn  *net.TCPConn
	alive bool
}

type req struct {
	Name      string
	TimeStamp time.Time
	Uuid      [16]byte
}

var lambda1 lambdaInstance
var lambda2 *lambdaInstance

func newLambdaInstance(name string) *lambdaInstance {
	return &lambdaInstance{name: name}
}

// server decode request from lambda1
func decode() req {
	reqBuff := make([]byte, 2048)
	n, err := lambda1.conn.Read(reqBuff)
	var v req
	err = binary.Unmarshal(reqBuff[:n], &v)
	if err != nil {
		fmt.Println("decode err is ", err)
	}
	return v
}

func main() {
	tcpAddr, err := net.ResolveTCPAddr("tcp", ":8080") // create TcpAddr
	if err != nil {
		fmt.Println(err)
		return
	}

	tcpListener, err := net.ListenTCP("tcp", tcpAddr) //Start listen
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Start listening", tcpAddr, "and waiting for client")

	for {
		lambda1.conn, err = tcpListener.AcceptTCP() //Accept lambda1
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("lambda1 Connect :", lambda1.conn.RemoteAddr())
		// get request struct
		//destination := decode()
		fmt.Println("destination is ", "Lambda2SmallJPG")

		if lambda2 == nil {
			lambda2 = newLambdaInstance("Lambda2SmallJPG")
		}
		if !lambda2.alive {
			fmt.Println("Lambda 2 is sleeping, need to activate")
			go lambdaTrigger(lambda2) // trigger lambda
			fmt.Println("Lambda 2 is active now")
			time.Sleep(2 * time.Second)
		}
		if lambda2.conn == nil {
			lambda2.conn, err = tcpListener.AcceptTCP() //Accept lambda2_simulator
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Println("Lamdba2 Connect :", lambda2.conn.RemoteAddr())
		}
		go receive1(&lambda1, lambda2)
		go receive2(lambda2, &lambda1)

	}

}

func receive1(l1 *lambdaInstance, l2 *lambdaInstance) {

	defer l1.conn.Close()
	reqBuff := make([]byte, 2048)
	//data, err := binary.Marshal(&request)
	//if err != nil {
	//	fmt.Println(err)
	//}

	// server read lambda1's request
	for {
		n, err := l1.conn.Read(reqBuff)
		if err == io.EOF {
			fmt.Println(l1.conn.RemoteAddr(), "has disconnect")
			break
		} else if err != nil {
			fmt.Println(l1.conn.RemoteAddr(), "connection error")
			return
		}
		fmt.Println("tcpConn1", l1.conn.RemoteAddr().String())

		// send lambda1's request to lambda2
		fmt.Println("data from lambda1 is ", reqBuff[:n])
		send(l2.conn, reqBuff[:n])
		fmt.Println("send to conn2")
	}
	fmt.Println("exit receive1")
}

func receive2(l2 *lambdaInstance, l1 *lambdaInstance) {

	fmt.Println("in the receive2")
	responseBuff := make([]byte, 2048)

	for {
		n, err := l2.conn.Read(responseBuff)
		//fmt.Println(content)
		if err == io.EOF {
			fmt.Println("lambda2 complete")
			l2.conn.Close()
			l2.conn = nil
			break
		}
		// send
		_, err = l1.conn.Write(responseBuff[:n])
		if err != nil {
			fmt.Println(err)
			return
		}
	}
	fmt.Println("right now in the receive2")
}

func send(conn net.Conn, object []byte) {
	_, err := conn.Write(object)
	if err != nil {
		fmt.Println(err)
	}
}

func lambdaTrigger(instance *lambdaInstance) {

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	client := lambda.New(sess, &aws.Config{Region: aws.String("us-east-1")})

	instance.alive = true
	_, err := client.Invoke(&lambda.InvokeInput{FunctionName: aws.String(instance.name)})
	if err != nil {
		fmt.Println("Error calling LambdaFunction")
	}
	instance.alive = false
}
