package main

import (
	"fmt"
	"github.com/bsm/redeo/resp"
	"github.com/wangaoone/redeo"
	"net"
)

var (
	chan1 = make(chan resp.CommandArgument, 1)
	chan2 = make(chan string, 1)
	count = 0
	cn    net.Conn
)

func main() {

	fmt.Println("receive from client...")
	srv := redeo.NewServer(nil)

	// Define handlers
	srv.Handle("echo", redeo.WrapperFunc(func(c *resp.Command) interface{} {
		if c.ArgN() != 1 {
			return redeo.ErrWrongNumberOfArgs(c.Name)
		}
		chan1 <- c.Arg(0)
		fmt.Println("out chan2")
		go sendtoLambda()
		return <-chan2
	}))

	// Open a new listener
	lis, err := net.Listen("tcp", ":3333")
	if err != nil {
		panic(err)
	}
	fmt.Println("start listening client face port 3333")

	defer lis.Close()

	// Start serving (blocking)
	err = srv.Serve(lis)
	if err != nil {
		fmt.Println(err)
	}

}

func sendtoLambda() {
	fmt.Println("ready send to lambda storage", <-chan1)
	fmt.Println("count is ", count)
	if count == 0 {
		srv := redeo.NewServer(nil)

		// Define handlers
		srv.HandleFunc("PING", func(w resp.ResponseWriter, _ *resp.Command) {
			w.AppendInlineString("PONG")
		})

		// Open a new listener
		lis, err := net.Listen("tcp", ":3000")
		if err != nil {
			panic(err)
		}
		defer lis.Close()

		fmt.Println("start listening lambda face port 3000")

		// client part
		cn = srv.Accept(lis)
		fmt.Println(cn.RemoteAddr())
		count = count + 1
	}
	fmt.Println("add is ", cn.LocalAddr())
	// writer and reader
	w := resp.NewRequestWriter(cn)
	r := resp.NewResponseReader(cn)

	w.WriteCmdString("ping")

	// Flush pipeline
	if err := w.Flush(); err != nil {
		fmt.Println(err)
	}
	// Consume responses
	s, _ := r.ReadInlineString()
	fmt.Println(s)
	chan2 <- s
	fmt.Println("send to lambda and get resposne over")
}

//func receiveFromClient() {
//	fmt.Println("receive from client...")
//	srv := redeo.NewServer(nil)
//
//	// Define handlers
//	srv.HandleFunc("ping", func(w resp.ResponseWriter, _ *resp.Command) {
//		w.AppendInlineString("POOOOOONG")
//	})
//
//	srv.Handle("echo", redeo.WrapperFunc(func(c *resp.Command) interface{} {
//		if c.ArgN() != 1 {
//			return redeo.ErrWrongNumberOfArgs(c.Name)
//		}
//		chan1 <- c.Arg(0)
//		fmt.Println("out chan2")
//		return <-chan2
//	}))
//
//	// Open a new listener
//	lis, err := net.Listen("tcp", ":3333")
//	if err != nil {
//		panic(err)
//	}
//	fmt.Println("start listening client face port 3333")
//
//	defer lis.Close()
//
//	go sendtoLambda()
//
//	// Start serving (blocking)
//	err = srv.Serve(lis)
//	if err != nil {
//		fmt.Println(err)
//	}
//}
