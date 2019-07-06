package main

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/bsm/redeo/resp"
	"github.com/wangaoone/redeo"
	"github.com/wangaoone/s3gof3r"
	"io"
	"net"
	"time"
)

var (
	//lambdaConn, _ = net.Dial("tcp", "52.201.234.235:6379") // t2.micro ec2 server
	lambdaConn, _ = net.Dial("tcp", "54.204.180.34:6379") // 10Gbps ec2 server
	srv           = redeo.NewServer(nil)
	myMap         = make(map[string][]byte)
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
				chunkId, _ := c.Arg(2).Int()
				key := c.Arg(3).String()

				//if obj == nil {
				//	obj = remoteGet("ao.webapp", key)
				//	myCache.Set(string(c.Arg(0)), obj, -1)
				//} else {
				//	fmt.Println("find key")
				//}
				t1 := time.Now()
				//val, err := myCache.Get(key)
				//if err == false {
				//	fmt.Println("not found")
				//}
				val, found := myMap[key]
				if found == false {
					fmt.Println("not found")
				}
				fmt.Println("cache time is", time.Since(t1))
				fmt.Println("item find", len(val))

				t2 := time.Now()
				// construct lambda store response
				t4 := time.Now()
				w.AppendBulkString(key)
				fmt.Println("append key", time.Since(t4))
				t5 := time.Now()
				w.AppendInt(clientId)
				fmt.Println("append client id", time.Since(t5))
				t6 := time.Now()
				w.AppendInt(reqId)
				fmt.Println("append request id", time.Since(t6))
				t7 := time.Now()
				w.AppendInt(chunkId)
				fmt.Println("append chunk id", time.Since(t7))
				t8 := time.Now()
				w.AppendBulk(val)
				fmt.Println("append val id", time.Since(t8))
				t9 := time.Now()
				if err := w.Flush(); err != nil {
					panic(err)
				}
				fmt.Println("flush", time.Since(t9))
				fmt.Println("writer append time is", time.Since(t2))

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

				//mu.Lock()
				//myCache.Set(key, val, -1)
				myMap[key] = val
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
				fmt.Println("set complete", "key:", key, "req id:,", reqId, "client id:", clientId)
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
