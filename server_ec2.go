package main

import (
	"net"
	"fmt"
	"strings"
)

func main(){
	tcpAddr, err := net.ResolveTCPAddr("tcp", ":8080") // create TcpAddr
	if err != nil{
		fmt.Println(err)
		return
	}

	tcpLinstener, err := net.ListenTCP("tcp", tcpAddr) //Start listen
	if err != nil{
		fmt.Println(err)
		return
	}
	fmt.Printf("Start listen:[%s]", tcpAddr)

  for {
    tcpCoon, err := tcpLinstener.AcceptTCP()  //Accept
    if err != nil{
  		fmt.Println(err)
  		return
  	}
    go handle(tcpCoon)
  }

	// defer tcpCoon.Close()
  //
	// data := make([]byte, 2048)
	// n, err := tcpCoon.Read(data)  //Client connect read data
	// if err != nil{
	// 	fmt.Println(err)
	// 	return
	// }
  //
	// recvStr := string(data[:n])
	// fmt.Println("Recv:", recvStr)
	// tcpCoon.Write([]byte(strings.ToUpper(recvStr)))  // write to client
}

func handle(conn net.Conn){
	defer conn.Close()  //关闭连接
	fmt.Println("Connect :", conn.RemoteAddr())

	for {
		//只要客户端没有断开连接，一直保持连接，读取数据
		data := make([]byte, 2048)
		n, err := conn.Read(data)
		//数据长度为0表示客户端连接已经断开
		if n == 0{
			fmt.Printf("%s has disconnect", conn.RemoteAddr())
			break
		}
		if err != nil{
			fmt.Println(err)
			continue
		}
		fmt.Printf("Receive data [%s] from [%s]", string(data[:n]), conn.RemoteAddr())

		rspData := strings.ToUpper(string(data[:n]))
		_, err = conn.Write([]byte(rspData))
		if err != nil{
			fmt.Println(err)
			continue
		}
	}

}
