package main

import (
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"log"
	"os"
	"strings"
)

func main() {
	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32, 3)
	_, size, addrList := client.GetArgs(os.Args)

	addrArr := strings.Split(addrList, ",")
	data := cli.GenerateSetData(size)

	// start dial and PUT/GET
	cli.Dial(addrArr)
	if _, _, ok := cli.MkSet("foo", data); !ok {
		log.Fatal("Failed to set")
		return
	}else{
		fmt.Println("successfull SET")
	}

	fmt.Println(cli.MkGet("foo", cli.GenerateSingleRandomGet(data)))
	return
}