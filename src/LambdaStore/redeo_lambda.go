package main

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/wangaoone/redeo"
	"github.com/wangaoone/redeo/resp"
	"github.com/wangaoone/s3gof3r"
	"io"
	"net"
	"time"
)

type chunk struct {
	id   string
	body []byte
}

var (
	lambdaConn, _ = net.Dial("tcp", "3.220.244.122:6379") // 10Gbps ec2 server UbuntuProxy0
	//lambdaConn, _ = net.Dial("tcp", "172.31.18.174:6379") // 10Gbps ec2 server Proxy1
	srv     = redeo.NewServer(nil)
	myMap   = make(map[string]chunk)
	isFirst = true
	//lock    sync.Mutex
)

func HandleRequest() {
	timeOut := time.Duration(20 * time.Second)
	if isFirst == true {
		go func() {
			fmt.Println("conn is", lambdaConn.LocalAddr(), lambdaConn.RemoteAddr())
			// Define handlers
			srv.HandleFunc("get", func(w resp.ResponseWriter, c *resp.Command) {
				t := time.Now()
				timeOut = time.Duration(20 * time.Second)
				fmt.Println("In GET handler")

				connId := c.Arg(0).String()
				reqId := c.Arg(1).String()
				key := c.Arg(3).String()

				//val, err := myCache.Get(key)
				//if err == false {
				//	fmt.Println("not found")
				//}
				chunk, found := myMap[key]
				if found == false {
					fmt.Println("not found")
				}
				//lock.Lock()
				// construct lambda store response
				t2 := time.Now()
				w.AppendBulkString(connId)
				time2 := time.Since(t2)
				w.AppendBulkString(reqId)
				t4 := time.Now()
				w.AppendBulkString(chunk.id)
				time4 := time.Since(t4)
				t5 := time.Now()
				w.AppendBulk(chunk.body)
				time5 := time.Since(t5)
				t6 := time.Now()
				if err := w.Flush(); err != nil {
					panic(err)
				}
				time6 := time.Since(t6)
				duration := time.Since(t)
				fmt.Println("GET complete, Key:",
					key, "connID: ", connId, "ChunkID: ", chunk.id, "Item found", len(chunk.body),
					"AppendClientId time is", time2,
					"AppendChunkId time is", time4,
					"AppendBody time is ", time5,
					"Flush time is ", time6,
					"Duration time is", duration)
				//lock.Unlock()
			})

			srv.HandleFunc("set", func(w resp.ResponseWriter, c *resp.Command) {
				t := time.Now()
				timeOut = time.Duration(20 * time.Second)
				fmt.Println("In SET handler")
				//if c.ArgN() != 3 {
				//	w.AppendError(redeo.WrongNumberOfArgs(c.Name))
				//	return
				//}

				connId := c.Arg(0).String()
				reqId := c.Arg(1).String()
				chunkId := c.Arg(2).String()
				key := c.Arg(3).String()
				val := c.Arg(4).Bytes()
				chunk := chunk{id: chunkId, body: val}

				myMap[key] = chunk
				// write Key, clientId, chunkId, body back to server
				//lock.Lock()
				w.AppendBulkString(connId)
				w.AppendBulkString(reqId)
				w.AppendBulkString(chunkId)
				w.AppendInt(1)
				if err := w.Flush(); err != nil {
					panic(err)
				}
				duration := time.Since(t)
				fmt.Println("SET complete, Key:",
					key, "connID: ", connId, "ChunkID: ", chunk.id, "Item found", len(chunk.body),
					"Duration time is", duration)
				//lock.Unlock()
			})
			srv.Serve_client(lambdaConn)
		}()
	}
	// timeout control
	select {
	case <-time.After(timeOut):
		fmt.Println("Lambda timeout, going to return function")
		isFirst = false
		return
	}
}

func remoteGet(bucket string, key string) []byte {
	fmt.Println("get from remote storage")
	k, err := s3gof3r.EnvKeys()
	if err != nil {
		fmt.Println(err)
	}

	s3 := s3gof3r.New("", k)
	b := s3.Bucket(bucket)

	reader, _, err := b.GetReader(key, nil)
	if err != nil {
		fmt.Println(err)
	}
	obj := streamToByte(reader)
	return obj
}

func streamToByte(stream io.Reader) []byte {
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(stream)
	if err != nil {
		fmt.Println(err)
	}
	return buf.Bytes()
}

func main() {
	lambda.Start(HandleRequest)
}
