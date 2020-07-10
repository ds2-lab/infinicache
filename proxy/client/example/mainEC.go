package main

import (
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"log"
	"math/rand"
	"os"
	"strings"
)

func main() {
	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32, 3)
	_, size, addrList := client.GetArgs(os.Args)

	// initial object with random value
	val := make([]byte, size)
	rand.Read(val)

	// parse server address
	addrArr := strings.Split(addrList, ",")


	cli.Dial(addrArr)

	key := fmt.Sprintf("k1")
	if _, stats, ok := cli.EcSet(key, val); !ok {
		log.Println("Failed to SET ", key)
	}else{
		log.Println("Successfull SET ", key, " ", stats)
	}

	if _, _, stats, ok := cli.EcGet(key, size); !ok {
		log.Println("Failed to GET ", key)
	} else {
		log.Println("Successfull GET ", key, " ", stats)
	}

	return
}
