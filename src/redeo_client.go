package main

import (
	"fmt"
	"github.com/bsm/redeo/resp"
	"net"
)

func main() {
	//cn, _ := net.Dial("tcp", "localhost:3333")
	cn, _ := net.Dial("tcp", "52.201.234.235:6378")
	defer cn.Close()

	// Wrap connection
	w := resp.NewRequestWriter(cn)
	r := resp.NewResponseReader(cn)

	// Write pipeline
	//w.WriteCmdString("get", "30mb.jpg")
	w.WriteCmdString("set", "key", "this is the set")

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
	//buf := make([]byte, 0)
	//c, err := r.ReadBulk(buf)
	//if err != nil {
	//	fmt.Println(err)
	//}
	//fmt.Println(len(c))

}
