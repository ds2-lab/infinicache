package ec2_server

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

//
//
//package main
//
//import (
//"fmt"
//"github.com/aws/aws-sdk-go/aws"
//"github.com/aws/aws-sdk-go/aws/session"
//"github.com/aws/aws-sdk-go/service/lambda"
//"io/ioutil"
//"log"
//"net"
//"net/http"
//"os"
//"time"
//)
//
//var tcpConn1 net.Conn
//var tcpConn2 net.Conn
//
//const BUFFERSIZE = 12800
//
//func main() {
//	tcpAddr, err := net.ResolveTCPAddr("tcp", ":3333") // create TcpAddr
//	if err != nil {
//		fmt.Println(err)
//		return
//	}
//
//	tcpLinstener, err := net.ListenTCP("tcp", tcpAddr) //Start listen
//	if err != nil {
//		fmt.Println(err)
//		return
//	}
//	fmt.Println("Start listening", tcpAddr, "and waiting for client")
//
//	for {
//		tcpConn1, err = tcpLinstener.AcceptTCP() //Accept lambda1
//		if err != nil {
//			fmt.Println(err)
//			return
//		}
//		fmt.Println("lambda1 Connect :", tcpConn1.RemoteAddr())
//
//		go receive(tcpConn1)
//
//		go lambdaTrigger("Lambda2SmallJPG")
//
//		tcpConn2, err = tcpLinstener.AcceptTCP() //Accept lambda2_simulator
//		if err != nil {
//			fmt.Println(err)
//			return
//		}
//		fmt.Println("lamdba2 Connect :", tcpConn2.RemoteAddr())
//
//		go receive2(tcpConn2)
//
//	}
//
//	// defer tcpCoon.Close()
//	//
//	// data := make([]byte, 2048)
//	// n, err := tcpCoon.Read(data)  //Client connect read data
//	// if err != nil{
//	// 	fmt.Println(err)
//	// 	return
//	// }
//	//
//	// recvStr := string(data[:n])
//	// fmt.Println("Recv:", recvStr)
//	// tcpCoon.Write([]byte(strings.ToUpper(recvStr)))  // write to client
//}
//
//func lambdaTrigger(funcName string) {
//	sess := session.Must(session.NewSessionWithOptions(session.Options{
//		SharedConfigState: session.SharedConfigEnable,
//	}))
//
//	client := lambda.New(sess, &aws.Config{Region: aws.String("us-east-1")})
//
//	_, err := client.Invoke(&lambda.InvokeInput{FunctionName: aws.String(funcName)})
//	if err != nil {
//		fmt.Println("Error calling MyGetItemsFunction")
//		os.Exit(0)
//	}
//}
//
//func receive(conn net.Conn) {
//
//	//lambdaTrigger("Lambda2SmallJPG")
//	time.Sleep(5 * time.Second)
//	data := make([]byte, 2048)
//	for {
//		// server read lambda1's request
//		n, err := conn.Read(data)
//		if n == 0 {
//			fmt.Println(conn.RemoteAddr(), "has disconnect")
//			break
//		}
//		if err != nil {
//			fmt.Println(conn.RemoteAddr(), "connection error")
//			return
//		}
//		fmt.Println("tcpConn1", conn.RemoteAddr().String(), "Receive data String:", string(data[:n]))
//
//		// send lambda1's request to lambda2_simulator
//		if conn == tcpConn1 {
//			rspData := string(data[:n])
//			fmt.Println("data from lambda1 is ", rspData)
//			send(tcpConn2, rspData)
//		}
//
//	}
//}
//
//func receive2(conn net.Conn) {
//	//data := make([]byte, 10)
//	//for {
//	//	// server read lambda2_simulator's response
//	//	n, err := conn.Read(data)
//	//	if n == 0 {
//	//		fmt.Println(conn.RemoteAddr(), "has disconnect")
//	//		break
//	//	}
//	//	if err != nil {
//	//		fmt.Println(conn.RemoteAddr(), "connection error")
//	//		return
//	//	}
//	//	fmt.Println("tcpConn2", conn.RemoteAddr().String(), "Receive data String:", string(data[:n]))
//	//
//	//	// send lambda2_simulator's response to lambda1
//	//
//	//	if conn == tcpConn2 {
//	//		rspData := string(data[:n])
//	//		fmt.Println("data from lambda2_simulator is ", rspData)
//	//		send(tcpConn1, rspData)
//	//	}
//	//
//	//}
//
//	// file transfer
//
//	// read
//	bufferFileName := make([]byte, 64)
//	bufferFileSize := make([]byte, 10)
//	bufferContent := make([]byte, BUFFERSIZE)
//
//	for {
//		name, err := conn.Read(bufferFileName)
//		if name == 0 {
//			fmt.Println(conn.RemoteAddr(), "has disconnect")
//			break
//		}
//		if err != nil {
//			fmt.Println(err)
//			return
//		}
//		size, err := conn.Read(bufferFileSize)
//		if err != nil {
//			fmt.Println(err)
//			return
//		}
//		content, err := conn.Read(bufferContent)
//		//if content == 0 {
//		//	fmt.Println(conn.RemoteAddr(), "has disconnect")
//		//	break
//		//}
//		if err != nil {
//			fmt.Println(err)
//			return
//		}
//		//fmt.Println("this is the content", string(content))
//
//		// send
//		if conn == tcpConn2 {
//			resName := string(bufferFileName[:name])
//			send(tcpConn1, resName)
//			resSize := string(bufferFileSize[:size])
//			send(tcpConn1, resSize)
//			resContent := string(bufferContent[:content])
//			send(tcpConn1, resContent)
//		}
//	}
//}
//
//func send(conn net.Conn, object string) {
//	//data := make([]byte, 2048)
//
//	//for {
//	//	n, err := conn.Read(data)
//	//	if n == 0 {
//	//		fmt.Println(conn.RemoteAddr(), "has disconnect")
//	//		break
//	//	}
//	//	if err != nil {
//	//		fmt.Println(conn.RemoteAddr(), "connection error")
//	//		return
//	//	}
//	//	fmt.Println(conn.RemoteAddr().String(), "Receive data String:", string(data[:n]))
//	//
//	//	// write lambda response to client
//	//	rspData := string(data[:n])
//	//	fmt.Println(rspData)
//	//	temp = rspData
//	//	_, err = conn.Write([]byte(rspData))
//	//	if err != nil {
//	//		fmt.Println(err)
//	//		continue
//	//	}
//	//}
//
//	_, err := conn.Write([]byte(object))
//	if err != nil {
//		fmt.Println(err)
//	}
//
//}
//
//func handle1(conn net.Conn) {
//	defer conn.Close() // close connect
//
//	data := make([]byte, 2048)
//
//	for {
//		//keep connecting
//		n, err := conn.Read(data)
//		// if data = 0 then disconnect
//		if n == 0 {
//			fmt.Println(conn.RemoteAddr(), "has disconnect")
//			break
//		}
//		if err != nil {
//			fmt.Println(err)
//			continue
//		}
//		fmt.Println("Receive data", string(data[:n]), "from", conn.RemoteAddr())
//
//		// Get Http Response from lambda
//		res, err := http.Get("https://1puql3o86l.execute-api.us-east-1.amazonaws.com/default/Hello")
//		if err != nil {
//			log.Fatal(err)
//			return
//		}
//		body, err := ioutil.ReadAll(res.Body)
//		res.Body.Close()
//		if err != nil {
//			log.Fatal(err)
//			break
//		}
//		//fmt.Println("Already invoke lambda function")
//
//		// write lambda response to client
//		rspData := body
//		_, err = conn.Write([]byte(rspData))
//		if err != nil {
//			fmt.Println(err)
//			continue
//		}
//	}
//
//}
//
//func handle(conn net.Conn) {
//	data := make([]byte, 2048)
//
//	for {
//		n, err := conn.Read(data)
//		if n == 0 {
//			fmt.Println(conn.RemoteAddr(), "has disconnect")
//			break
//		}
//		if err != nil {
//			fmt.Println(conn.RemoteAddr(), "connection error")
//			return
//		}
//		fmt.Println(conn.RemoteAddr().String(), "Receive data String:", string(data[:n]))
//
//		//Get Http Response from lambda
//		res, err := http.Get("https://1puql3o86l.execute-api.us-east-1.amazonaws.com/default/Hello")
//
//		//tr := &http.Transport{
//		//	MaxIdleConns:       10,
//		//	IdleConnTimeout:    30 * time.Second,
//		//	DisableCompression: true,
//		//}
//		//client := &http.Client{Transport: tr}
//		//res, err := client.Get("https://1puql3o86l.execute-api.us-east-1.amazonaws.com/default/Hello")
//
//		if err != nil {
//			log.Fatal(err)
//			return
//		}
//		robots, err := ioutil.ReadAll(res.Body)
//		res.Body.Close()
//		if err != nil {
//			log.Fatal(err)
//			break
//		}
//		fmt.Println("%s", robots)
//
//		// write lambda response to client
//		rspData := robots
//		_, err = conn.Write([]byte(rspData))
//		if err != nil {
//			fmt.Println(err)
//			continue
//		}
//	}
//}


