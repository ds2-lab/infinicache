package main

import (
	"fmt"
	"net"

	"github.com/bsm/redeo/resp"
)

func main() {
	cn, _ := net.Dial("tcp", "localhost:3333")
	//cn, _ := net.Dial("tcp", "52.201.234.235:6378")
	defer cn.Close()

	// Wrap connection
	w := resp.NewRequestWriter(cn)
	r := resp.NewResponseReader(cn)

	// Write pipeline
	w.WriteCmdString("echo", "hellp")

	// Flush pipeline
	if err := w.Flush(); err != nil {
		panic(err)
	}

	// Consume responses
	t, err := r.PeekType()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(t)
	c, err := r.ReadBulkString()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(c)

}
