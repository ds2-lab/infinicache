package temp

import (
	"context"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"io"
	"net"
	"os"
	"strconv"
)

func handler(ctx context.Context, s3Event events.S3Event) {
	for _, record := range s3Event.Records {
		s3 := record.S3
		fmt.Printf("[%s - %s] Bucket = %s, Key = %s \n", record.EventSource, record.EventTime, s3.Bucket.Name, s3.Object.Key)
	}

	file, err := os.Open("test.jpg")
	checkerror(err)
	fileInfo, err := file.Stat();
	checkerror(err)
	fmt.Println("already read file", fileInfo)

	tcpAddr, err := net.ResolveTCPAddr("tcp", "52.201.234.235:8080")  // ec2 address
	//tcpAddr, err := net.ResolveTCPAddr("tcp", "localhost:3333")
	if err != nil {
		fmt.Println(err)
		return
	}

	tcpCoon, err := net.DialTCP("tcp", nil, tcpAddr) //dial tcp
	if err != nil {
		fmt.Println(err)
		return
	}
	defer tcpCoon.Close() //close conn
	fmt.Println("Start receiving data...")

	// receive lambda1's request
	recvData := make([]byte, 2048)
	n, err := tcpCoon.Read(recvData) // read data
	if err != nil {
		fmt.Println(err)
		return
	}
	recvStr := string(recvData[:n])
	fmt.Println("The request from lambda1 is\n", recvStr)

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

	fileSize := fillString(strconv.FormatInt(fileInfo.Size(), 10), 10)
	fileName := fillString(fileInfo.Name(), 64)
	tcpCoon.Write([]byte(fileSize))
	tcpCoon.Write([]byte(fileName))

	sendBuffer := make([]byte, 1024)
	fmt.Println("Start sending file")
	for {
		_, err = file.Read(sendBuffer)
		//fmt.Println("buffer ", string(sendBuffer))
		if err == io.EOF {
			fmt.Println("err == EOF")
			break
		}
		tcpCoon.Write(sendBuffer)
	}
	//_,err = file.Read(sendBuffer)
	//checkerror(err)
	//tcpCoon.Write(sendBuffer)
	fmt.Println("sended")
	//fmt.Println("buffer ", string(sendBuffer))

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

func main() {
	lambda.Start(handler)
}
