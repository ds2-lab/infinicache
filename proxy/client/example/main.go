package main

import (
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"log"
	"math/rand"
	"strings"
)

func main() {
	addrList := "10.4.0.100:6378"
	// initial object with random value
	val := make([]byte, 1024)
	rand.Read(val)

	// parse server address
	addrArr := strings.Split(addrList, ",")

	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32, 3)

	// start dial and PUT/GET
	cli.Dial(addrArr)
	if _, ok := cli.EcSet("foo", val); !ok {
		log.Fatal("Failed to set")
		return
	}

	if _, _, ok := cli.EcGet("foo", 1024); !ok {
		log.Fatal("Failed to get")
	} else {
		fmt.Println("Successfull GET")
	}
	return
}
