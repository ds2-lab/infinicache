package main

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/bsm/redeo/resp"
	"github.com/patrickmn/go-cache"
	"github.com/wangaoone/redeo"
	"github.com/wangaoone/s3gof3r"
	"io"
	"net"
	"time"
)

var (
	srv           = redeo.NewServer(nil)
	lambdaConn, _ = net.Dial("tcp", "52.201.234.235:6379")
	t             = time.Now()
	myCache       = cache.New(5*time.Minute, 10*time.Minute)
)

func HandleRequest() {
	go func() {
		fmt.Println("time is ", t)
		fmt.Println("conn is", lambdaConn.LocalAddr(), lambdaConn.RemoteAddr())

		// Define handlers
		srv.HandleFunc("get", func(w resp.ResponseWriter, c *resp.Command) {
			fmt.Println("in the get function")

			key := string(c.Arg(0))
			obj, _ := myCache.Get(key)
			if obj == nil {
				obj = remoteGet("ao.webapp", key)
				myCache.Set(string(c.Arg(0)), obj, -1)
			} else {
				fmt.Println("find key")
			}
			w.AppendBulk(obj.([]uint8))
		})

		srv.HandleFunc("set", func(w resp.ResponseWriter, c *resp.Command) {
			if c.ArgN() != 2 {
				w.AppendError(redeo.WrongNumberOfArgs(c.Name))
				return
			}

			key := c.Arg(0).String()
			val := c.Arg(1).String()

			//mu.Lock()
			myCache.Set(key, val, -1)
			//mu.Unlock()
			temp, err := myCache.Get(key)
			if temp == nil {
				fmt.Println("set failed", err)
			}
			fmt.Println("set complete, result is ", key, myCache.ItemCount())
			w.AppendInt(1)
		})
		srv.Serve_client(lambdaConn)
	}()
	
	// timeout control
	select {
	case <-time.After(10 * time.Second):
		fmt.Println("Lambda timeout, going to return function")
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
