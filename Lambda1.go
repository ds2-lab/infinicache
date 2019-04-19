package main

import (
	"context"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type MyEvent struct {
	Name string `json:"name"`
}

func lambda1(ctx context.Context, name MyEvent) {
	startDial := time.Now()
	tcpAddr, err := net.ResolveTCPAddr("tcp", "52.201.234.235:8080") // ec2 address
	//tcpAddr, err := net.ResolveTCPAddr("tcp", "localhost:3333")
	checkerror(err)

	tcpCoon, err := net.DialTCP("tcp", nil, tcpAddr) //dial tcp
	checkerror(err)
	defer tcpCoon.Close() //close conn

	// send request to server
	sendData := "I want data"
	n, err := tcpCoon.Write([]byte(sendData)) // send data
	checkerror(err)
	fmt.Println("Send", n, "byte data successed", "message is\n", sendData)

	// receive response from vm <--- lambda2_simulator
	//recvData := make([]byte, 10)
	//n, err = tcpCoon.Read(recvData) // read data
	//checkerror(err)
	//recvStr := string(recvData[:n])
	//fmt.Println("The data from lambda2_simulator is\n", recvStr)
	//response := time.Since(startDial)
	//fmt.Println("RTT is ", response)

	//receive file
	bufferFileName := make([]byte, 64)
	bufferFileSize := make([]byte, 10)
	tcpCoon.Read(bufferFileSize)
	tcpCoon.Read(bufferFileName)

	fileSize, _ := strconv.ParseInt(strings.Trim(string(bufferFileSize), ":"), 10, 64)
	//fileName := strings.Trim(string(bufferFileName), ":")
	fileName := "Fig.jpg"
	fmt.Println("name", fileName, "size", fileSize)

	newFile, err := os.Create("/tmp/Fig.jpg")
	checkerror(err)
	defer newFile.Close()
	fmt.Println("create file succeed")
	var receivedBytes int64

	for {
		if (fileSize - receivedBytes) < 1024 {
			io.CopyN(newFile, tcpCoon, (fileSize - receivedBytes))
			tcpCoon.Read(make([]byte, (receivedBytes+1024)-fileSize))
			break
		}
		io.CopyN(newFile, tcpCoon, 1024)
		receivedBytes += 1024
	}
	//io.CopyN(newFile, tcpCoon, (fileSize - receivedBytes))
	//tcpCoon.Read(make([]byte, (receivedBytes+1024)-fileSize))
	fmt.Println("Received file completely!\n", string(bufferFileSize), "\n", string(bufferFileName))
	response := time.Since(startDial)
	fmt.Println("RTT is ", response)

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
