package main

import (
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/bsm/redeo/resp"
	"github.com/wangaoone/redeo"
	"net"
)

func HandleRequest() {
	cn, err := net.Dial("tcp", "52.201.234.235:6379")
	if err != nil {
		fmt.Println(err)
	}
	defer cn.Close()
	srv := redeo.NewServer(nil)

	// Define handlers
	srv.HandleFunc("ping", func(w resp.ResponseWriter, _ *resp.Command) {
		w.AppendInlineString("PONG")
	})
	srv.HandleFunc("info", func(w resp.ResponseWriter, _ *resp.Command) {
		w.AppendInlineString("this is from lambda")
	})
	fmt.Println("conn is", cn.RemoteAddr())

	srv.Serve_client(cn)
}

func main() {
	lambda.Start(HandleRequest)
}
