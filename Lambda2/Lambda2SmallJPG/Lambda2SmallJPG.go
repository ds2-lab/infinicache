package main

import (
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"io"
	"net"
	"os"
	"strconv"
	"time"
)

const BUFFERSIZE = 12800

//var (
//	tcpAddr *net.TCPAddr
//	tcpConn net.Conn
//)
//
//func init() {
//	tcpAddr, _ = net.ResolveTCPAddr("tcp", "52.201.234.235:8080") // ec2 address
//	tcpConn, _ = net.DialTCP("tcp", nil, tcpAddr)                 //dial tcp
//}
func HandleRequest() {
	tcpAddr, err := net.ResolveTCPAddr("tcp", "52.201.234.235:8080") // ec2 address
	if err != nil {
		fmt.Println(err)
		return
	}

	// dial tcp establish connection socket2
	tcpConn, err := net.DialTCP("tcp", nil, tcpAddr) //dial tcp
	if err != nil {
		fmt.Println(err)
		return
	}
	//defer tcpConn.Close() //close conn
	fmt.Println("Start receiving data...")
	for {

		// load file first
		Object, err := os.Open("1mb.jpg")
		checkerror(err)
		fileInfo, err := Object.Stat()
		//checkerror(err)
		fmt.Println("already read file", fileInfo)

		// receive lambda1's request
		recvData := make([]byte, 2048)
		n, err := tcpConn.Read(recvData) // read lambda1's request
		if err != nil {
			fmt.Println(err)
			return
		}

		recvDataTimeStamp := time.Now()
		recvStr := string(recvData[:n])
		fmt.Println("The request from lambda1 is:", recvStr)
		fmt.Println("the recv timestamp is", recvDataTimeStamp)
		fmt.Println("start sending file")

		// send response to server
		//Object := "this is lambda2_simulator"
		//n, err = tcpCoon.Write([]byte(Object))
		//// send data
		//if err != nil {
		//	fmt.Println(err)
		//	return
		//}
		//fmt.Println("Send", n, "byte data successed", "message is\n", Object)

		// send file
		//for {
		fileSize := fillString(strconv.FormatInt(fileInfo.Size(), 10), 10)
		fileName := fillString(fileInfo.Name(), 64)
		tcpConn.Write([]byte(fileSize))
		tcpConn.Write([]byte(fileName))

		sendObject(tcpConn, Object)

		//tcpConn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	}

}

func fillString(retunString string, toLength int) string {
	for {
		lengtString := len(retunString)
		if lengtString < toLength {
			retunString = retunString + ":"
			continue
		}
		break
	}
	return retunString
}

func checkerror(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
func sendObject(conn net.Conn, obj *os.File) {
	sendBuffer := make([]byte, BUFFERSIZE)
	//fmt.Println("Start sending file")
	for {
		_, err := obj.Read(sendBuffer)
		//fmt.Println("buffer ", string(sendBuffer))
		if err == io.EOF {
			fmt.Println("no dat left")
			break
		}
		conn.Write(sendBuffer)
	}
	fmt.Println("sended")
}

func main() {

	lambda.Start(HandleRequest)
}
