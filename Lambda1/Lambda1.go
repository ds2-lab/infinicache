package main

import (
	"bytes"
	"fmt"
	"github.com/google/uuid"
	"github.com/kelindar/binary"
	"net"
	"time"
)

type req struct {
	Name      string
	TimeStamp time.Time
	Uuid      [16]byte
}

type response struct {
	Length int
	Obj    []byte
	Uuid   [16]byte
}

func encode(name string) []byte {
	myReq := &req{
		Name:      name,
		TimeStamp: time.Now(),
		Uuid:      uuid.New(),
	}
	encoded, err := binary.Marshal(myReq)
	if err != nil {
		fmt.Println("encode err", err)
	}
	return encoded
}

func connect() *net.TCPConn {
	tcpAddr, err := net.ResolveTCPAddr("tcp", "52.201.234.235:8080") // ec2 address
	if err != nil {
		fmt.Println(err)
		return nil
	}

	tcpConn, err := net.DialTCP("tcp", nil, tcpAddr) //dial tcp
	if err != nil {
		fmt.Println(err)
		return nil
	}
	return tcpConn
}

func main() {
	tcpCoon := connect()
	defer tcpCoon.Close() //close conn

	// send request to server
	encodedReq := encode("Lambda2SmallJPG")
	n, err := tcpCoon.Write(encodedReq) // send data
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Send", n, "byte data successed", "message is\n", encodedReq)

	// receive from server
	var b bytes.Buffer
	//buff := make([]byte, 2048)
	//bs := make([]byte, 4)
	//n, err = tcpCoon.Read(bs)
	//if err != nil {
	//	fmt.Println(err)
	//	return
	//}
	//s := binary.LittleEndian.Uint32(bs[:n])
	//fmt.Printf("s is %d\n", int(s))
	//length := 0
	infoBuff := make([]byte, 2048)
	fileBuff := make([]byte, 2048)
	n, err = tcpCoon.Read(infoBuff)
	if err != nil {
		fmt.Println(err)
		return
	}
	var info response
	count := 0
	_ = binary.Unmarshal(infoBuff[:n], &info)
	length := info.Length
	fmt.Println("total size:", length)

	for {
		n, err := tcpCoon.Read(fileBuff)
		//fmt.Println(n)
		count += n
		fmt.Println("length", count)
		if n > 0 {
			b.Write(fileBuff[:n])
		}
		if count >= length {
			break
		} else if err != nil {
			fmt.Println("err is ", err)
			break
		}
	}
	fmt.Println("total size:", length)

}
