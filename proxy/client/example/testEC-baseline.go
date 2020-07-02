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
	var setStats []float64
	var getStats []float64
	cli.Dial(addrArr)

	for k:=0; k<=1000; k++{
		key := fmt.Sprintf("k%d", k)

		for l:=0; l<9; l++ {
			key := fmt.Sprintf("%s%d", key, l)
			if _, stats, ok := cli.EcSet(key, val); !ok {
				log.Println("Failed to SET ", key)
			}else{
				log.Println("Successfull SET ", key)
				setStats = append(setStats, stats)
			}
		}

	}

	for k:=0; k<=1000; k++{
		key := fmt.Sprintf("k%d", k)

		for l:=0; l<9; l++ {
			key := fmt.Sprintf("%s%d", key, l)
			if _, _, stats, ok := cli.EcGet(key, 160); !ok {
				log.Println("Failed to GET ", key)
			} else {
				log.Println("Successfull GET ", key)
				getStats = append(getStats, stats)
			}
		}

	}
	log.Println("Average SET time: ", cli.Average(setStats))
	log.Println("Average GET time: ", cli.Average(getStats))
	return
}
