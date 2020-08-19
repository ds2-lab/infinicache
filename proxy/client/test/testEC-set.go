package main

import (
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"log"
	"os"
	"strings"
	"math/rand"
)

func main() {
	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32, 3)
	requestsNumber, size, addrList := client.GetArgs(os.Args)

	val := make([]byte, size)
	rand.Read(val)
	// parse server address
	addrArr := strings.Split(addrList, ",")

	var setStats []float64
	cli.Dial(addrArr)

	for k:=0; k<=requestsNumber; k++{
		key := fmt.Sprintf("k.%d", k)
		if _, stats, ok := cli.EcSet(key, val); !ok {
			log.Println("Failed to SET ", key)
		}else{
			log.Println("Successfull SET ", key)
			setStats = append(setStats, stats)
		}
	}

	sMin, sMax, sAvg, sSd, sPercentiles := cli.GetStats(setStats)
	log.Println("SET stats ", sMin, sMax, sAvg, sSd, sPercentiles)
	return
}

