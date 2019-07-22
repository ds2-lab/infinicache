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
	id   int64
	body []byte
}

var (
	//lambdaConn, _ = net.Dial("tcp", "54.204.180.34:6379") // 10Gbps ec2 server Proxy0
	lambdaConn, _ = net.Dial("tcp", "172.31.18.174:6379") // 10Gbps ec2 server Proxy1
	srv           = redeo.NewServer(nil)
	myMap         = make(map[string]chunk)
	isFirst       = true
)

func HandleRequest() {
	if isFirst == true {
		go func() {
			fmt.Println("conn is", lambdaConn.LocalAddr(), lambdaConn.RemoteAddr())
			// Define handlers
			srv.HandleFunc("get", func(w resp.ResponseWriter, c *resp.Command) {
				t := time.Now()
				fmt.Println("in the get function")

				clientId, _ := c.Arg(0).Int()
				key := c.Arg(3).String()

				//val, err := myCache.Get(key)
				//if err == false {
				//	fmt.Println("not found")
				//}
				chunk, found := myMap[key]
				if found == false {
					fmt.Println("not found")
				}
				fmt.Println("item find", len(chunk.body))

				// construct lambda store response
				t2 := time.Now()
				w.AppendInt(clientId)
				fmt.Println("appendClientId time is", time.Since(t2))
				t4 := time.Now()
				w.AppendInt(chunk.id)
				fmt.Println("appendChunkId time is", time.Since(t4))
				t5 := time.Now()
				w.AppendBulk(chunk.body)
				fmt.Println("appendBody time is ", time.Since(t5))
				t6 := time.Now()
				if err := w.Flush(); err != nil {
					panic(err)
				}
				fmt.Println("flush time is ", time.Since(t6))
				fmt.Println("duration time is", time.Since(t),
					"get complete", "key:", key, "client id:", clientId, "chunk id is", chunk.id)
			})

			srv.HandleFunc("set", func(w resp.ResponseWriter, c *resp.Command) {
				fmt.Println("in the set function")
				//if c.ArgN() != 3 {
				//	w.AppendError(redeo.WrongNumberOfArgs(c.Name))
				//	return
				//}

				clientId, _ := c.Arg(0).Int()
				chunkId, _ := c.Arg(2).Int()
				key := c.Arg(3).String()
				val := c.Arg(4).Bytes()
				chunk := chunk{id: chunkId, body: val}

				myMap[key] = chunk
				// write Key, clientId, chunkId, body back to server
				w.AppendInt(clientId)
				w.AppendInt(chunkId)
				w.AppendInt(1)
				if err := w.Flush(); err != nil {
					panic(err)
				}
				fmt.Println("set complete", "key:", key, "val len", len(val), "client id:", clientId, "chunk id:", chunkId)
			})
			srv.Serve_client(lambdaConn)
		}()
	}
	// timeout control
	select {
	case <-time.After(120 * time.Second):
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
