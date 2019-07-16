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
	//lambdaConn, _ = net.Dial("tcp", "52.201.234.235:6379") // t2.micro ec2 server
	lambdaConn, _ = net.Dial("tcp", "54.204.180.34:6379") // 10Gbps ec2 server
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
				reqId, _ := c.Arg(1).Int()
				//_, _ = c.Arg(2).Int()
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
				w.AppendBulkString(key)
				w.AppendInt(clientId)
				w.AppendInt(reqId)
				w.AppendInt(chunk.id)
				w.AppendBulk(chunk.body)
				if err := w.Flush(); err != nil {
					panic(err)
				}
				fmt.Println("duration time is", time.Since(t),
					"get complete", "key:", key, "req id:", reqId, "client id:", clientId)
			})

			srv.HandleFunc("set", func(w resp.ResponseWriter, c *resp.Command) {
				fmt.Println("in the set function")
				//if c.ArgN() != 3 {
				//	w.AppendError(redeo.WrongNumberOfArgs(c.Name))
				//	return
				//}

				clientId, _ := c.Arg(0).Int()
				reqId, _ := c.Arg(1).Int()
				chunkId, _ := c.Arg(2).Int()
				key := c.Arg(3).String()
				val := c.Arg(4).Bytes()
				fmt.Println("client Id:", clientId, "req Id:", reqId, "chunk id:", chunkId)
				chunk := chunk{id: chunkId, body: val}

				//mu.Lock()
				//myCache.Set(key, val, -1)
				myMap[key] = chunk
				//gc.Set(key, val)
				//mu.Unlock()
				//temp, err := myCache.Get(key)
				//temp, err := gc.Get(key)
				//if temp == nil {
				//	fmt.Println("set failed", err)
				//}
				//fmt.Println("set complete", key, val, clientId)
				// write Key, clientId, chunkId, body back to server
				w.AppendBulkString(key)
				w.AppendInt(clientId)
				w.AppendInt(reqId)
				w.AppendInt(chunkId)
				w.AppendInt(1)
				if err := w.Flush(); err != nil {
					panic(err)
				}
				fmt.Println("set complete", "key:", key, "val len", len(val), "req id:,", reqId, "client id:", clientId)
			})
			srv.Serve_client(lambdaConn)
		}()
	}
	// timeout control
	select {
	case <-time.After(60 * time.Second):
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
