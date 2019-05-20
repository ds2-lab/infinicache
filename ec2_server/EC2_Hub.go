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

type header struct {
	Name      string
	TimeStamp time.Time
	Uuid      [16]byte
}

type Frame struct {
	lenHeader uint32
	lenBody   uint32
	header    []byte
	body      []byte
}

var myLambda lambdaInstance
var lambda2 *lambdaInstance
var frameHandler = make(chan *Frame, 100)

func newLambdaInstance(name string) *lambdaInstance {
	return &lambdaInstance{
		name:  name,
		conn:  nil,
		alive: false,
	}
}

func decodeHeader(encodeBuff []byte) header {
	var s header
	err := binary.Unmarshal(encodeBuff, &s)
	if err != nil {
		fmt.Println(err)
	}
	return s
}

func parseHeader(receive *Frame) header {
	myHeader := decodeHeader(receive.header)
	return myHeader
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

	//for {
	//	lambda1.conn, err = tcpListener.AcceptTCP() //Accept lambda1
	//	if err != nil {
	//		fmt.Println(err)
	//		return
	//	}
	//	fmt.Println("lambda1 Connect :", lambda1.conn.RemoteAddr())
	//
	//	// get request struct
	//	//destination := decode()
	//	fmt.Println("destination is ", "Lambda2SmallJPG")
	//
	//	if lambda2 == nil {
	//		lambda2 = newLambdaInstance("Lambda2SmallJPG")
	//	}
	//	if !lambda2.alive {
	//		fmt.Println("Lambda 2 is sleeping, need to activate")
	//		go lambdaTrigger(lambda2) // trigger lambda
	//		fmt.Println("Lambda 2 is active now")
	//		time.Sleep(2 * time.Second)
	//	}
	//	if lambda2.conn == nil {
	//		lambda2.conn, err = tcpListener.AcceptTCP() //Accept lambda2_simulator
	//		if err != nil {
	//			fmt.Println(err)
	//			return
	//		}
	//		fmt.Println("Lamdba2 Connect :", lambda2.conn.RemoteAddr())
	//	}
	//	go receive1(&lambda1, lambda2)
	//	go receive2(lambda2, &lambda1)
	//}

	for {
		myLambda.conn, err = tcpListener.AcceptTCP()
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("lambda connect: ", myLambda.conn.RemoteAddr())

		go receive(myLambda)

		receiveFrame := <-frameHandler
		receiveHeader := parseHeader(receiveFrame)
		fmt.Println("receiveHeader is ", receiveHeader)
		targetLambda := receiveHeader.Name

		if lambda2 == nil {
			lambda2 = newLambdaInstance(targetLambda)
		}
		if !lambda2.alive {
			fmt.Println("Lambda 2 is not alive, need to activate")
			go lambdaTrigger(lambda2)
			fmt.Println("Lambda 2 is active now")
		}
		if lambda2.conn == nil {
			lambda2.conn, err = tcpListener.AcceptTCP() //Accept lambda2_simulator
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Println("Lamdba2 Connect :", lambda2.conn.RemoteAddr())
		}

		tcpSend(lambda2.conn, receiveFrame)

		go receive(*lambda2)
		receiveFrame = <-frameHandler
		fmt.Println("body length is ", receiveFrame.lenBody)

		tcpSend(myLambda.conn, receiveFrame)

	}

}

func tcpSend(conn *net.TCPConn, frame *Frame) {

	myHeaderLen := make([]byte, 4)
	binary.BigEndian.PutUint32(myHeaderLen, uint32(frame.lenHeader)) // get header length

	myBodyLen := make([]byte, 4)
	binary.BigEndian.PutUint32(myBodyLen, uint32(frame.lenBody)) // get body length

	sendCount := 0
	switch sendCount {
	case 0:
		_, err := conn.Write(myHeaderLen)
		if err != nil {
			fmt.Println(err)
		}
		sendCount = 1
		fallthrough
	case 1:
		_, err := conn.Write(myBodyLen)
		if err != nil {
			fmt.Println(err)
		}
		sendCount = 2
		fallthrough
	case 2:
		_, err := conn.Write(frame.header)
		if err != nil {
			fmt.Println(err)
		}
		sendCount = 3
		fallthrough
	case 4:
		_, err := conn.Write(frame.body)
		if err != nil {
			fmt.Println(err)
		}
	}

}
func receive(l lambdaInstance) {
	time.Sleep(1 * time.Second)
	//defer l.conn.Close()
	frame := &Frame{
		lenHeader: 0,
		lenBody:   0,
		header:    []byte{},
		body:      []byte{},
	}
	intBuff := make([]byte, 4)
	fieldFrame := 0

	buff := make([]byte, 2048)
	for {
		buffFrom := 0
		n, err := l.conn.Read(buff)
		if err == io.EOF {
			fmt.Println(l.conn.RemoteAddr(), "has disconnect")
			break
		} else if err != nil {
			fmt.Println(l.conn.RemoteAddr(), "connection error")
			break
		}

		switch fieldFrame {
		case 0:
			// get header length
			if n-buffFrom >= cap(intBuff) {
				frame.lenHeader = binary.BigEndian.Uint32(buff[buffFrom : buffFrom+4])
				buffFrom = buffFrom + 4
				fieldFrame = 1
				goto nextCase0
			} else {
				copy(intBuff, buff[buffFrom:n])
			}
		nextCase0:
			fallthrough
		case 1:
			// get body length
			if n-buffFrom >= cap(intBuff) {
				frame.lenBody = binary.BigEndian.Uint32(buff[buffFrom : buffFrom+4])
				buffFrom = buffFrom + 4
				fieldFrame = 2
				goto nextCase1
			} else {
				copy(intBuff, buff[buffFrom:n])
			}
		nextCase1:
			fallthrough
		case 2:
			// get header
			if n-buffFrom >= int(frame.lenHeader)-len(frame.header) {
				frame.header = merge(frame.header, buff[buffFrom:buffFrom+int(frame.lenHeader)])
				buffFrom = buffFrom + len(frame.header)
				fieldFrame = 3
				buffFrom = buffFrom + int(frame.lenHeader) - len(frame.header)
				goto nextCase2
			} else {
				frame.header = merge(frame.header, buff[buffFrom:])
			}
		nextCase2:
			fallthrough
		case 3:
			// get body
			if n-buffFrom >= int(frame.lenBody)-len(frame.body) {
				frame.body = merge(frame.body, buff[buffFrom:buffFrom+int(frame.lenBody)])
				buffFrom = buffFrom + int(frame.lenBody) - len(frame.body)
				//frameHandler <- frame
				fieldFrame = 0
			} else {
				frame.body = merge(frame.body, buff[buffFrom:])
			}
			frameHandler <- frame
		}
	}
}

func lambdaTrigger(instance *lambdaInstance) {

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	client := lambda.New(sess, &aws.Config{Region: aws.String("us-east-1")})

	instance.alive = true
	_, err := client.Invoke(&lambda.InvokeInput{FunctionName: aws.String(instance.name)})
	fmt.Println(instance.name)
	if err != nil {
		fmt.Println("Error calling LambdaFunction")
	}
	fmt.Println("lamdba deactive")
	instance.alive = false
}

func merge(a []byte, b []byte) []byte {
	c := make([]byte, len(a)+len(b))
	copy(c, a)
	copy(c[len(a):], b)
	return c
}
