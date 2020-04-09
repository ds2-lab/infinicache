package main

import (
	"math/rand"
	"strings"

	"github.com/neboduus/infinicache/proxy/client"
)

var addrList = "35.204.109.185:6378"

func main() {
	// initial object with random value
	var val []byte
	val = make([]byte, 1024)
	rand.Read(val)

	// parse server address
	addrArr := strings.Split(addrList, ",")

	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32)

	// start dial and PUT/GET
	cli.Dial(addrArr)
	cli.EcSet("foo", val)
	cli.EcGet("foo", 1024)

}
