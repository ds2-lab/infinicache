package main

import (
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"log"
	"strings"
)

func main() {
	var addrList = "10.4.0.100:6378"

	// parse server address
	addrArr := strings.Split(addrList, ",")

	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32, 3)
	data := cli.GenerateSetData()
	// start dial and PUT/GET
	cli.Dial(addrArr)
	if _, _, ok := cli.MkSet("foo", data); !ok {
		log.Fatal("Failed to set")
		return
	}else{
		fmt.Println("successfull SET")
	}

	var keys [3]client.KVGetGroup
	keys[0] = client.KVGetGroup{Keys: []string{"k1", "k2"}}
	keys[1] = client.KVGetGroup{Keys: []string{"k4"}}
	keys[2] = client.KVGetGroup{Keys: []string{"k5"}}

	fmt.Println(cli.MkGet("foo", keys))
	return
}