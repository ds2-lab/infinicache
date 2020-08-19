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

	var getStats []float64
	cli.Dial(addrArr)


	for k:=0; k<=requestsNumber; k++{
		key := fmt.Sprintf("k.%d", k)

		if _, _, stats, ok := cli.EcGet(key, 1313); !ok {
			log.Println("Failed to GET ", key)
		} else {
			log.Println("Successfull GET ", key)
			getStats = append(getStats, stats)
		}



	}

	gMin, gMax, gAvg, gSd, gPercentiles := cli.GetStats(getStats)
	log.Println("GET stats ", gMin, gMax, gAvg, gSd, gPercentiles)
	return
}

