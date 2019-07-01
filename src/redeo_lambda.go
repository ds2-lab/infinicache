package main

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/bluele/gcache"
	"github.com/bsm/redeo/resp"
	"github.com/patrickmn/go-cache"
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
	myCache       = cache.New(60*time.Minute, 60*time.Minute)
	isFirst       = true
	gc            = gcache.New(20).LRU().Build()
)

func HandleRequest() {
	if isFirst == true {
		go func() {
			fmt.Println("conn is", lambdaConn.LocalAddr(), lambdaConn.RemoteAddr())

			// Define handlers
			srv.HandleFunc("get", func(w resp.ResponseWriter, c *resp.Command) {
				fmt.Println("in the get function")

				id := c.Arg(0).String()
				key := c.Arg(0).String()

				//if obj == nil {
				//	obj = remoteGet("ao.webapp", key)
				//	myCache.Set(string(c.Arg(0)), obj, -1)
				//} else {
				//	fmt.Println("find key")
				//}
				val, err := myCache.Get(key)
				if err == false {
					fmt.Println("not found")
				}
				fmt.Println("item find", len(val.([]byte)))
				// construct lambda store response
				//obj := newResponse(id, value.(string))
				//res, _ := binary.Marshal(obj)
				w.AppendBulkString(id)
				w.AppendBulkString(val.(string))
				if err := w.Flush(); err != nil {
					panic(err)
				}
			})

			srv.HandleFunc("set", func(w resp.ResponseWriter, c *resp.Command) {
				//if c.ArgN() != 3 {
				//	w.AppendError(redeo.WrongNumberOfArgs(c.Name))
				//	return
				//}

				clientId := c.Arg(0).String()
				chunkId, _ := c.Arg(1).Int()
				key := c.Arg(2).String()
				val := c.Arg(3).Bytes()

				//mu.Lock()
				myCache.Set(key, val, -1)
				//gc.Set(key, val)
				//mu.Unlock()
				//temp, err := myCache.Get(key)
				//temp, err := gc.Get(key)
				//if temp == nil {
				//	fmt.Println("set failed", err)
				//}
				fmt.Println("set complete", key, val, clientId)
				// write clientId, chunkId, Key, body back to server
				w.AppendBulkString(clientId)
				w.AppendInt(chunkId)
				w.AppendBulkString(key)
				//w.AppendBulkString("1")
				w.AppendBulk([]byte{1, 2, 3})
				if err := w.Flush(); err != nil {
					panic(err)
				}
			})
			srv.Serve_client(lambdaConn)
		}()
	}
	// timeout control
	select {
	case <-time.After(10 * time.Second):
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
