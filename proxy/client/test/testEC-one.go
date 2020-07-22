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
	println(requestsNumber,size,addrList)

	val := make([]byte, size)
	rand.Read(val)
	// parse server address
	addrArr := strings.Split(addrList, ",")

	var setStats []float64
	var getStats []float64
	cli.Dial(addrArr)

	for k:=0; k<=requestsNumber; k++{
		key := fmt.Sprintf("k%d", k)

		var s float64 = 0
		if _, stats, ok := cli.EcSet(key, val); !ok {
			log.Println("Failed to SET ", key)
		}else{
			log.Println("Successfull SET ", key)
			s += stats
		}
		if s!=0{
			setStats = append(setStats, s)
		}

	}

	for k:=0; k<=requestsNumber; k++{
		key := fmt.Sprintf("k%d", k)

		var s float64 = 0
		if _, _, stats, ok := cli.EcGet(key, size); !ok {
			log.Println("Failed to GET ", key)
		} else {
			log.Println("Successfull GET ", key)
			s += stats
		}

		if s != 0{
			getStats = append(getStats, s)
		}

	}

	log.Println(len(setStats), len(getStats))
	sMin, sMax, sAvg, sSd, sPercentiles := cli.GetStats(setStats)
	log.Println("SET stats ", sMin, sMax, sAvg, sSd, sPercentiles)
	gMin, gMax, gAvg, gSd, gPercentiles := cli.GetStats(getStats)
	log.Println("GET stats ", gMin, gMax, gAvg, gSd, gPercentiles)
	return
}

