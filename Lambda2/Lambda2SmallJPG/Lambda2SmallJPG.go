package main

import (
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/kelindar/binary"
	"io"
	"io/ioutil"
	"net"
	"os"
	"time"
)

var (
	cache        map[string][]byte
	srv          *server
	reqLambda1   header
	frameHandler = make(chan *Frame, 100)
)

type ret struct {
	n   int
	err error
}

type server struct {
	conn    *net.TCPConn
	closed  chan struct{}
	timer   *time.Timer
	ret     chan *ret
	reading bool
	buffer  []byte
}

type header struct {
	Name      string
	TimeStamp time.Time
	Uuid      [16]byte
}

type Frame struct {
	lenHeader uint32
	lenBody   uint32
	header    []byte
	body      []byte
}

type response struct {
	Length int
	Uuid   [16]byte
}

func newResponse(length int, uuid [16]byte) []byte {
	myRes := &response{
		Length: length,
		Uuid:   uuid,
	}
	encoded, err := binary.Marshal(myRes)
	if err != nil {
		fmt.Println("encode err", err)
	}
	return encoded
}

func newServer() *server {
	return &server{
		ret:    make(chan *ret),
		buffer: make([]byte, 2048),
	}
}

func (srv *server) start() {
	srv.closed = make(chan struct{})
	srv.timer = time.NewTimer(10 * time.Second)
	go func() {
		<-srv.timer.C
		fmt.Println("timer is triggered. Timed out")
		srv.stop()
	}()
}

func (srv *server) stop() {
	select {
	case <-srv.closed:
	// already closed
	default:
		fmt.Println("closing srv")
		close(srv.closed)
	}
}

// release resource
func (srv *server) close() {
	fmt.Println("closing")
	srv.timer.Stop()
	//close(srv.ret)
}

func (srv *server) recv() {
	if srv.reading {
		return
	}
	srv.reading = true
	frame := &Frame{
		lenHeader: 0,
		lenBody:   0,
		header:    []byte{},
		body:      []byte{},
	}
	intBuff := make([]byte, 4)
	fieldFrame := 0
	for {
		buffFrom := 0
		n, err := srv.conn.Read(srv.buffer)
		if err == io.EOF {
			fmt.Println(srv.conn.RemoteAddr(), "has disconnect")
			break
		} else if err != nil {
			fmt.Println(srv.conn.RemoteAddr(), "connection error")
			break
		}

		switch fieldFrame {
		case 0:
			// get header length
			if n-buffFrom >= cap(intBuff) {
				frame.lenHeader = binary.BigEndian.Uint32(srv.buffer[buffFrom : buffFrom+4])
				buffFrom = buffFrom + 4
				fieldFrame = 1
				goto nextCase0
			} else {
				copy(intBuff, srv.buffer[buffFrom:n])
			}
		nextCase0:
			fallthrough
		case 1:
			// get body length
			if n-buffFrom >= cap(intBuff) {
				frame.lenBody = binary.BigEndian.Uint32(srv.buffer[buffFrom : buffFrom+4])
				buffFrom = buffFrom + 4
				fieldFrame = 2
				goto nextCase1
			} else {
				copy(intBuff, srv.buffer[buffFrom:n])
			}
		nextCase1:
			fallthrough
		case 2:
			// get header
			if n-buffFrom >= int(frame.lenHeader)-len(frame.header) {
				frame.header = merge(frame.header, srv.buffer[buffFrom:buffFrom+int(frame.lenHeader)])
				buffFrom = buffFrom + len(frame.header)
				fieldFrame = 3
				buffFrom = buffFrom + int(frame.lenHeader) - len(frame.header)
				goto nextCase2
			} else {
				frame.header = merge(frame.header, srv.buffer[buffFrom:])
			}
		nextCase2:
			fallthrough
		case 3:
			// get body
			if n-buffFrom >= int(frame.lenBody)-len(frame.body) {
				frame.body = merge(frame.body, srv.buffer[buffFrom:buffFrom+int(frame.lenBody)])
				buffFrom = buffFrom + int(frame.lenBody) - len(frame.body)
				//frameHandler <- frame
				fieldFrame = 0
			} else {
				frame.body = merge(frame.body, srv.buffer[buffFrom:])
			}
			frameHandler <- frame
			srv.ret <- &ret{n, err}
			srv.reading = false
		}
	}

}

func HandleRequest() {
	srv.start()
	// release resource
	defer srv.close()

	if srv.conn == nil {
		srv.conn = connect()
	}

	fmt.Println("Start receiving data...")
	readObject("1mb.jpg")
	fmt.Println("already read file")

	for {
		fmt.Println("selecting")
		go srv.recv()
		select {
		case <-srv.closed:
			fmt.Println("srv closed")
			return
		// receive lambda1's request
		case readRet := <-srv.ret:
			if readRet.err != nil {
				fmt.Println("err is ", readRet.err)
			}

			// send file
			fmt.Println("start sending file")
			sendFrame := <-frameHandler
			sendFrame.body = getKey("1mb.jpg")
			sendFrame.lenBody = uint32(len(sendFrame.body))

			tcpSend(srv.conn, sendFrame)
			fmt.Println("write done")
		}
	}
}

func checkerror(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func readObject(obj string) {
	data, err := ioutil.ReadFile(obj)
	if err != nil {
		checkerror(err)
	} else {
		cache[obj] = data
	}
}

func getKey(obj string) []byte {
	_, exist := cache[obj]
	if !exist {
		readObject(obj)
	}
	return cache[obj]
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

func merge(a []byte, b []byte) []byte {
	c := make([]byte, len(a)+len(b))
	copy(c, a)
	copy(c[len(a):], b)
	return c
}

func tcpSend(conn *net.TCPConn, frame *Frame) {
	myHeaderLen := make([]byte, 4)
	binary.BigEndian.PutUint32(myHeaderLen, uint32(len(frame.header))) // get header length

	myBodyLen := make([]byte, 4)
	binary.BigEndian.PutUint32(myBodyLen, uint32(len(frame.body))) // get body length

	fmt.Println("send header len", myHeaderLen)
	fmt.Println("send body len", myBodyLen)

	sendCount := 0
	switch sendCount {
	case 0:
		_, err := conn.Write(myHeaderLen)
		if err != nil {
			fmt.Println(err)
		}
		sendCount = 1
		fallthrough
	case 1:
		_, err := conn.Write(myBodyLen)
		if err != nil {
			fmt.Println(err)
		}
		sendCount = 2
		fallthrough
	case 2:
		_, err := conn.Write(frame.header)
		if err != nil {
			fmt.Println(err)
		}
		sendCount = 3
		fallthrough
	case 4:
		_, err := conn.Write(frame.body)
		if err != nil {
			fmt.Println(err)
		}
	}
	fmt.Println("send complete")
}

func main() {
	cache = make(map[string][]byte)
	srv = newServer()
	lambda.Start(HandleRequest)
}
