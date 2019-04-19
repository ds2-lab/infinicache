package main

import (
        "fmt"
        "context"
        "github.com/aws/aws-lambda-go/lambda"
        "net"
)

type MyEvent struct {
        Name string `json:"name"`
}

func HandleRequest(ctx context.Context, name MyEvent) (string, error) {
        tcpAddr, err := net.ResolveTCPAddr("tcp", "52.201.234.235:8080")  //TCP连接地址
        if err != nil{
          fmt.Println(err)
          return "rr", err
        }

        tcpCoon, err := net.DialTCP("tcp", nil, tcpAddr)  //建立连接
        if err != nil{
          fmt.Println(err)
          return "rr", err
        }
        defer tcpCoon.Close()  //关闭

        sendData := "helloworld"
        n, err := tcpCoon.Write([]byte(sendData))  //发送数据
        if err != nil{
          fmt.Println(err)
          return "rr", err
        }
        fmt.Printf("Send %d byte data success: %s", n, sendData)

        recvData := make([]byte, 2048)
        n, err = tcpCoon.Read(recvData)  //读取数据
        if err != nil{
          fmt.Println(err)
          return "rr", err
        }
        recvStr := string(recvData[:n])
        return fmt.Sprintf("Response data: %s", recvStr), nil
        // return fmt.Sprintf("Hello %s!", name.Name ), nil
}


func main() {
        lambda.Start(HandleRequest)
}
