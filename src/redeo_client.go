package main

import (
	"fmt"
	"net"

	"github.com/bsm/redeo/resp"
)

func main() {
	//cn, _ := net.Dial("tcp", "localhost:3333")
	cn, _ := net.Dial("tcp", "52.201.234.235:6378")
	defer cn.Close()

	// Wrap connection
	w := resp.NewRequestWriter(cn)
	r := resp.NewResponseReader(cn)

	// Write pipeline
	w.WriteCmdString("PING")

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
