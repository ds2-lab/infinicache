package main

import (
	"fmt"
	"net"
)

var tcpConn1 net.Conn
var tcpConn2 net.Conn

const BUFFERSIZE = 12800

func main() {
	tcpAddr, err := net.ResolveTCPAddr("tcp", ":8080") // create TcpAddr
	if err != nil {
		fmt.Println(err)
		return
	}

	tcpLinstener, err := net.ListenTCP("tcp", tcpAddr) //Start listen
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Start listening", tcpAddr, "and waiting for client")

	for {

		tcpConn2, err = tcpLinstener.AcceptTCP() //Accept lambda2_simulator
		if err != nil {
			fmt.Println(err)
			return
		}
		//tcpCoon.SetReadDeadline(time.Now().Add(time.Duration(10) * time.Second))

		fmt.Println("lamdba2 Connect :", tcpConn2.RemoteAddr())
		//go handle(tcpCoon)

		go receive2(tcpConn2)

		tcpConn1, err = tcpLinstener.AcceptTCP() //Accept lambda1
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("lambda1 Connect :", tcpConn1.RemoteAddr())
		//fmt.Println("start send")

		go receive(tcpConn1)

	}
}

func receive(conn net.Conn) {
	data := make([]byte, 2048)
	for {
		// server read lambda1's request
		n, err := conn.Read(data)
		if n == 0 {
			fmt.Println(conn.RemoteAddr(), "has disconnect")
			break
		}
		if err != nil {
			fmt.Println(conn.RemoteAddr(), "connection error")
			return
		}
		fmt.Println("tcpConn1", conn.RemoteAddr().String(), "Receive data String:", string(data[:n]))

		// send lambda1's request to lambda2_simulator
		if conn == tcpConn1 {
			rspData := string(data[:n])
			fmt.Println("data from lambda1 is ", rspData)
			send(tcpConn2, rspData)
		}

	}
}

func receive2(conn net.Conn) {
	// file transfer

	// read
	bufferFileName := make([]byte, 64)
	bufferFileSize := make([]byte, 10)
	bufferContent := make([]byte, BUFFERSIZE)

	for {
		name, err := conn.Read(bufferFileName)
		if name == 0 {
			fmt.Println(conn.RemoteAddr(), "has disconnect")
			break
		}
		if err != nil {
			fmt.Println(err)
			return
		}
		size, err := conn.Read(bufferFileSize)
		if err != nil {
			fmt.Println(err)
			return
		}
		content, err := conn.Read(bufferContent)
		//if content == 0 {
		//	fmt.Println(conn.RemoteAddr(), "has disconnect")
		//	break
		//}
		if err != nil {
			fmt.Println(err)
			return
		}
		//fmt.Println("this is the content", string(content))

		// send
		if conn == tcpConn2 {
			resName := string(bufferFileName[:name])
			send(tcpConn1, resName)
			resSize := string(bufferFileSize[:size])
			send(tcpConn1, resSize)
			resContent := string(bufferContent[:content])
			send(tcpConn1, resContent)
		}
	}
}

func send(conn net.Conn, object string) {
	_, err := conn.Write([]byte(object))
	if err != nil {
		fmt.Println(err)
	}
}
