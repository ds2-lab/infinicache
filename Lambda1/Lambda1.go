package main

import (
	"bytes"
	"fmt"
	"github.com/google/uuid"
	"github.com/kelindar/binary"
	"net"
	"time"
)

type header struct {
	Name      string
	TimeStamp time.Time
	Uuid      [16]byte
}

type body struct {
	obj []byte
}

func encodeHeader(name string) []byte {
	myHeader := &header{
		Name:      name,
		TimeStamp: time.Now(),
		Uuid:      uuid.New(),
	}
	encoded, err := binary.Marshal(myHeader)
	if err != nil {
		fmt.Println("encode err", err)
	}
	return encoded
}

func encodeBody() []byte {
	myBody := &body{
		obj: []byte{1, 2, 3},
	}
	encoded, err := binary.Marshal(myBody)
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

func tcpSend(conn *net.TCPConn, headerLen []byte, bodyLen []byte, header []byte, body []byte) {
	fmt.Println(headerLen)
	fmt.Println(bodyLen)
	fmt.Println(header)
	fmt.Println(body)

	sendCount := 0
	switch sendCount {
	case 0:
		_, err := conn.Write(headerLen)
		if err != nil {
			fmt.Println(err)
		}
		sendCount = 1
		fallthrough
	case 1:
		_, err := conn.Write(bodyLen)
		if err != nil {
			fmt.Println(err)
		}
		sendCount = 2
		fallthrough
	case 2:
		_, err := conn.Write(header)
		if err != nil {
			fmt.Println(err)
		}
		sendCount = 3
		fallthrough
	case 4:
		_, err := conn.Write(body)
		if err != nil {
			fmt.Println(err)
		}
	}

}

func main() {
	tcpCoon := connect()
	defer tcpCoon.Close() //close conn

	myHeader := encodeHeader("Lambda2SmallJPG")
	myBody := encodeBody()

	myHeaderLen := make([]byte, 4)
	myBodyLen := make([]byte, 4)

	binary.BigEndian.PutUint32(myHeaderLen, uint32(len(myHeader))) // get header length
	binary.BigEndian.PutUint32(myBodyLen, uint32(len(myBody)))     // get body length

	tcpSend(tcpCoon, myHeaderLen, myBodyLen, myHeader, myBody)

	// receive from server
	var b bytes.Buffer
	var info header
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
	infoBuff := make([]byte, 4)
	bodyLenBuff := make([]byte, 4)
	bodyBuff := make([]byte, 2048)

	n, err := tcpCoon.Read(infoBuff) // conn read header
	if err != nil {
		fmt.Println(err)
		return
	}
	_ = binary.Unmarshal(infoBuff[:n], &info)

	n, err = tcpCoon.Read(bodyLenBuff) // conn read BodyLen
	if err != nil {
		fmt.Println(err)
		return
	}
	count := 0
	length := n

	for {
		n, err := tcpCoon.Read(bodyBuff)
		//fmt.Println(n)
		count += n
		fmt.Println("length", count)
		if n > 0 {
			b.Write(bodyBuff[:n])
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
