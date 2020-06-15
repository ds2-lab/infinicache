package main

import (
	"bytes"
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"log"
	"strings"
)

func main() {
	var addrList = "10.4.0.100:6378"
	// initial object with random value
	var val []byte
	val = []byte("test")

	// parse server address
	addrArr := strings.Split(addrList, ",")

	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32, 3)

	// start dial and PUT/GET
	cli.Dial(addrArr)
	if _, ok := cli.RSet("foo", val); !ok {
		log.Fatal("Failed to set")
		return
	}

	if _, reader, ok := cli.RGet("foo", len(val)); !ok {
		log.Fatal("Failed to get")
		return
	} else {
		buf := new(bytes.Buffer)
		buf.ReadFrom(reader)
		reader.Close()
		s := buf.String()
		fmt.Println("received value: ")
		fmt.Println(s)
	}
}