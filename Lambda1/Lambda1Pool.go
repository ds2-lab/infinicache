package main

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/silenceper/pool"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const BUFFERSIZE = 12800

func lambda1() {

	// pool config
	factory := func() (interface{}, error) { return net.Dial("tcp", "52.201.234.235:8080") }
	close := func(v interface{}) error { return v.(net.Conn).Close() }
	poolConfig := &pool.Config{
		InitialCap:  1,
		MaxCap:      30,
		Factory:     factory,
		Close:       close,
		IdleTimeout: 60 * time.Second,
	}
	p, err := pool.NewChannelPool(poolConfig)
	if err != nil {
		fmt.Println("err=", err)
	}
	v, err := p.Get()
	tcpCoon := v.(net.Conn)
	defer tcpCoon.Close()
	startDial := time.Now()
	//tcpAddr, err := net.ResolveTCPAddr("tcp", "52.201.234.235:8080")  // ec2 address
	////tcpAddr, err := net.ResolveTCPAddr("tcp", "localhost:3333")
	//checkerror(err)
	//
	//tcpCoon, err := net.DialTCP("tcp", nil, tcpAddr) //dial tcp
	//checkerror(err)
	//defer tcpCoon.Close() //close conn

	// send request to server
	sendData := "I want data"
	n, err := tcpCoon.Write([]byte(sendData)) // send data
	checkerror(err)
	fmt.Println("Send", n, "byte data successed", "message is\n", sendData)

	//receive file
	bufferFileName := make([]byte, 64)
	bufferFileSize := make([]byte, 10)
	tcpCoon.Read(bufferFileSize)
	tcpCoon.Read(bufferFileName)

	fileSize, _ := strconv.ParseInt(strings.Trim(string(bufferFileSize), ":"), 10, 64)
	fileName := strings.Trim(string(bufferFileName), ":")
	//fileName := "Fig.jpg"
	fmt.Println("name", fileName, "size", fileSize)

	//newFile, err := os.Create(fileName)
	//checkerror(err)
	//defer newFile.Close()
	var buf bytes.Buffer
	var receivedBytes int64

	//for {
	//	if (fileSize - receivedBytes) < 1024 {
	//		io.CopyN(newFile, tcpCoon, (fileSize - receivedBytes))
	//		tcpCoon.Read(make([]byte, (receivedBytes+1024)-fileSize))
	//		break
	//	}
	//	io.CopyN(newFile, tcpCoon, 1024)
	//	receivedBytes += 1024
	//}
	for {
		if (fileSize - receivedBytes) < BUFFERSIZE {
			io.CopyN(&buf, tcpCoon, (fileSize - receivedBytes))
			tcpCoon.Read(make([]byte, (receivedBytes+BUFFERSIZE)-fileSize))
			break
		}
		io.CopyN(&buf, tcpCoon, BUFFERSIZE)
		receivedBytes += BUFFERSIZE
	}
	fmt.Println("total size:", buf.Len())

	//io.CopyN(newFile, tcpCoon, (fileSize - receivedBytes))
	//tcpCoon.Read(make([]byte, (receivedBytes+1024)-fileSize))
	fmt.Println("Received file completely!\n", string(bufferFileSize), "\n", string(bufferFileName))
	response := time.Since(startDial)
	fmt.Println("RTT is ", response)
	fmt.Println("Strat writing buffer to file")

	//file, _ := os.Create("Fig.jpg")
	//buf.WriteTo(file)
	//或者使用写入，fmt.Fprintf(file,buf.String())
}

func checkerror(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func main() {
	lambda.Start(lambda1)
}
