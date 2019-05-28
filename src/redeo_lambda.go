package src

import (
	"fmt"
	"github.com/bsm/redeo/resp"
	"github.com/wangaoone/redeo"
	"net"
)

func main() {
	cn, err := net.Dial("tcp", "localhost:3000")
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
