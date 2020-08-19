package main

import (
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"log"
	"os"
	"strings"
)

func main() {
	requestsNumber, size, addrList := client.GetArgs(os.Args)

	// parse server address
	addrArr := strings.Split(addrList, ",")

	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32, 3)

	// start dial and PUT/GET
	cli.Dial(addrArr)
	var data [][]client.KVSetGroup

	var setStats []float64

	for k:=0; k<requestsNumber; k++{
		d := cli.GenerateSetData2(size, 3)
		data = append(data, d)
		key := fmt.Sprintf("HighLevelKey-%d", k)
		if _, stats, ok := cli.MkSet(key, d); !ok {
			log.Println("Failed to mkSET", key)
		}else{
			setStats = append(setStats, stats)
			log.Println("Successfull mkSET", key)
		}
	}

	sMin, sMax, sAvg, sSd, sPercentiles := cli.GetStats(setStats)
	log.Println("SET stats ", sMin, sMax, sAvg, sSd, sPercentiles)
	return

}