package main

import (
	"github.com/neboduus/infinicache/proxy/client"
	"log"
	"os"
	"strconv"
	"strings"
	"math/rand"
)

func main() {
	requestsNumber, size, addrList :=client.GetArgs(os.Args)
	val := make([]byte, size)
	rand.Read(val)

	// parse server address
	addrArr := strings.Split(addrList, ",")

	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32, 3)

	// start dial and PUT/GET
	cli.Dial(addrArr)
	var setStats []float64

	for k:=0; k<requestsNumber; k++{
		key := "foo" + strconv.Itoa(k)
		if _, stats, ok := cli.RSet(key, val); !ok {
			log.Println("Failed to set ", key)
		}else{
			log.Println("Succesfully rSET", key)
			setStats = append(setStats, stats)
		}
	}

	sMin, sMax, sAvg, sSd, sPercentiles := cli.GetStats(setStats)
	log.Println("SET stats ", sMin, sMax, sAvg, sSd, sPercentiles)

}

