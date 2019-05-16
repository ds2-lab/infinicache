package main

import (
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/kelindar/binary"
	"io/ioutil"
	"net"
	"os"
	"time"
)

var (
	cache      map[string][]byte
	srv        *server
	reqLambda1 req
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

type req struct {
	Name      string
	TimeStamp time.Time
	Uuid      [16]byte
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
	n, err := srv.conn.Read(srv.buffer)
	if err != nil {
		fmt.Println("recv err is ", err)
	}
	fmt.Println("buff is ", srv.buffer)

	srv.ret <- &ret{n, err}
	srv.reading = false
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
			recv := srv.buffer[:readRet.n]
			_ = binary.Unmarshal(recv, &reqLambda1)

			// send file
			fmt.Println("start sending file")
			obj := getKey("1mb.jpg")
			fmt.Println("The request from lambda1 is:", reqLambda1.Name)
			objInfo := newResponse(len(obj), reqLambda1.Uuid)
			// send response to server
			srv.conn.Write(objInfo)
			time.Sleep(2 * time.Second)
			srv.conn.Write(obj)
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

func main() {
	cache = make(map[string][]byte)
	srv = newServer()
	lambda.Start(HandleRequest)
}
