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

		var s float64 = 0
		for l:=0; l<3; l++ {
			key := fmt.Sprintf("%s.%d", key, l)
			if _, _, stats, ok := cli.EcGet(key, 1313); !ok {
				log.Println("Failed to GET ", key)
			} else {
				log.Println("Successfull GET ", key)
				s += stats
			}
		}
		if s!=0{
			getStats = append(getStats, s)
		}

	}

	gMin, gMax, gAvg, gSd, gPercentiles := cli.GetStats(getStats)
	log.Println("GET stats ", gMin, gMax, gAvg, gSd, gPercentiles)
	return
}

