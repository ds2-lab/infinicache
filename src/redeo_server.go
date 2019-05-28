package src

import (
	"fmt"
	"github.com/bsm/redeo/resp"
	"github.com/wangaoone/redeo"
	"net"
)

func main() {
	go sendtoLambda()
	receiveFromClient()

}

func sendtoLambda() {
	srv := redeo.NewServer(nil)

	// Define handlers
	srv.HandleFunc("PING", func(w resp.ResponseWriter, _ *resp.Command) {
		w.AppendInlineString("PONG")
	})
	srv.HandleFunc("info", func(w resp.ResponseWriter, _ *resp.Command) {
		w.AppendInlineString("this is from lambda")
	})

	// Open a new listener
	lis, err := net.Listen("tcp", ":3000")
	if err != nil {
		panic(err)
	}
	defer lis.Close()

	// client part
	cn := srv.Accept(lis)
	fmt.Println(cn.RemoteAddr())

	// writer and reader
	w := resp.NewRequestWriter(cn)
	r := resp.NewResponseReader(cn)
	w.WriteCmdString("ping")
	w.WriteCmdString("INFO")
	// Flush pipeline
	if err := w.Flush(); err != nil {
		panic(err)
	}
	// Consume responses
	for i := 0; i < 5; i++ {
		t, err := r.PeekType()
		if err != nil {
			return
		}

		switch t {
		case resp.TypeInline:
			s, _ := r.ReadInlineString()
			fmt.Println(s)
		case resp.TypeBulk:
			s, _ := r.ReadBulkString()
			fmt.Println(s)
		case resp.TypeInt:
			n, _ := r.ReadInt()
			fmt.Println(n)
		case resp.TypeNil:
			_ = r.ReadNil()
			fmt.Println(nil)
		default:
			panic("unexpected response type")
		}
	}
}
func receiveFromClient() {
	srv := redeo.NewServer(nil)

	// Define handlers
	srv.HandleFunc("ping", func(w resp.ResponseWriter, _ *resp.Command) {
		w.AppendInlineString("PONG")
	})
	srv.HandleFunc("info", func(w resp.ResponseWriter, _ *resp.Command) {
		w.AppendInlineString("from proxy")
	})

	// More handlers; demo usage of redeo.WrapperFunc
	srv.Handle("echo", redeo.WrapperFunc(func(c *resp.Command) interface{} {
		if c.ArgN() != 1 {
			return redeo.ErrWrongNumberOfArgs(c.Name)
		}
		return c.Arg(0)
	}))

	// Open a new listener
	lis, err := net.Listen("tcp", ":3333")
	if err != nil {
		panic(err)
	}
	defer lis.Close()

	// Start serving (blocking)
	srv.Serve(lis)
}
