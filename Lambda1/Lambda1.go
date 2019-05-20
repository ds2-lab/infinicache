package main

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/kelindar/binary"
	"io"
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

type Frame struct {
	lenHeader uint32
	lenBody   uint32
	header    []byte
	body      []byte
}

var frameHandler = make(chan *Frame, 100)

func encodeHeader(funcName string) []byte {
	myHeader := &header{
		Name:      funcName,
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
	//tcpAddr, err := net.ResolveTCPAddr("tcp", "localhost:3333") // ec2 address
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
	//fmt.Println(headerLen)
	//fmt.Println(bodyLen)
	//fmt.Println(header)
	//fmt.Println(body)

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
	binary.BigEndian.PutUint32(myHeaderLen, uint32(len(myHeader))) // get header length

	myBodyLen := make([]byte, 4)
	binary.BigEndian.PutUint32(myBodyLen, uint32(len(myBody))) // get body length

	tcpSend(tcpCoon, myHeaderLen, myBodyLen, myHeader, myBody)

	// receive from server
	go receive(tcpCoon)
	fmt.Println(<-frameHandler)
}
func receive(conn *net.TCPConn) {
	time.Sleep(1 * time.Second)
	//defer conn.Close()
	frame := &Frame{
		lenHeader: 0,
		lenBody:   0,
		header:    []byte{},
		body:      []byte{},
	}
	intBuff := make([]byte, 4)
	fieldFrame := 0

	buff := make([]byte, 2048)
	for {
		buffFrom := 0
		n, err := conn.Read(buff)
		if err == io.EOF {
			fmt.Println(conn.RemoteAddr(), "has disconnect")
			break
		} else if err != nil {
			fmt.Println(conn.RemoteAddr(), "connection error")
			break
		}

		switch fieldFrame {
		case 0:
			// get header length
			if n-buffFrom >= cap(intBuff) {
				frame.lenHeader = binary.BigEndian.Uint32(buff[buffFrom : buffFrom+4])
				buffFrom = buffFrom + 4
				fieldFrame = 1
				goto nextCase0
			} else {
				copy(intBuff, buff[buffFrom:n])
			}
		nextCase0:
			fallthrough
		case 1:
			// get body length
			if n-buffFrom >= cap(intBuff) {
				frame.lenBody = binary.BigEndian.Uint32(buff[buffFrom : buffFrom+4])
				buffFrom = buffFrom + 4
				fieldFrame = 2
				goto nextCase1
			} else {
				copy(intBuff, buff[buffFrom:n])
			}
		nextCase1:
			fallthrough
		case 2:
			// get header
			if n-buffFrom >= int(frame.lenHeader)-len(frame.header) {
				frame.header = merge(frame.header, buff[buffFrom:buffFrom+int(frame.lenHeader)])
				buffFrom = buffFrom + len(frame.header)
				fieldFrame = 3
				buffFrom = buffFrom + int(frame.lenHeader) - len(frame.header)
				goto nextCase2
			} else {
				frame.header = merge(frame.header, buff[buffFrom:])
			}
		nextCase2:
			fallthrough
		case 3:
			// get body
			if n-buffFrom >= int(frame.lenBody)-len(frame.body) {
				frame.body = merge(frame.body, buff[buffFrom:buffFrom+int(frame.lenBody)])
				buffFrom = buffFrom + int(frame.lenBody) - len(frame.body)
				//frameHandler <- frame
				fieldFrame = 0
			} else {
				frame.body = merge(frame.body, buff[buffFrom:])
			}
		}
		frameHandler <- frame
	}
}
func merge(a []byte, b []byte) []byte {
	c := make([]byte, len(a)+len(b))
	copy(c, a)
	copy(c[len(a):], b)
	return c
}
