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
	val := make([]byte, 160)
	rand.Read(val)

	// parse server address
	addrArr := strings.Split(addrList, ",")

	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32, 3)
	var setStats []float32
	var getStats []float32
	cli.Dial(addrArr)

	for k:=0; k<=500; k++{
		key := fmt.Sprintf("k%d", k)
		if _, stats, ok := cli.EcSet(key, val); !ok {
			log.Println("Failed to SET ", key)
		}else{
			log.Println("Successfull SET ", key)
			setStats = append(setStats, stats)
		}

	}

	for k:=0; k<=500; k++{
		key := fmt.Sprintf("k%d", k)
		if _, _, stats, ok := cli.EcGet("foo", 160); !ok {
			log.Println("Failed to GET ", key)
		} else {
			log.Println("Successfull GET ", key)
			setStats = append(getStats, stats)
		}
	}
	log.Println("Average SET time: ", cli.Average(setStats))
	log.Println("Average GET time: ", cli.Average(getStats))
	return
}
