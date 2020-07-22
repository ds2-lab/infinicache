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

	requestsNumber, size, addrList := client.GetArgs(os.Args)
	// parse server address
	addrArr := strings.Split(addrList, ",")

	// start dial and PUT/GET
	cli.Dial(addrArr)
	var data [][3]client.KVSetGroup

	var setStats []float64
	var getStats []float64

	for k:=0; k<requestsNumber; k++{
		d := cli.GenerateSetData(size)
		data = append(data, d)
		key := fmt.Sprintf("HighLevelKey-%d", k)
		if _, stats, ok := cli.MkSet(key, d); !ok {
			log.Println("Failed to mkSET", key)
		}else{
			setStats = append(setStats, stats)
			log.Println("Successfull mkSET", key)
		}
	}

	getData := cli.GenerateRandomGet(data)

	for k:=0; k<len(getData); k++{
		d := getData[k]
		key := fmt.Sprintf("HighLevelKey-%d", k)
		if res, stats, ok := cli.MkGet(key, d); !ok {
			log.Println("Failed to mkGET %v", d)
		}else{
			getStats = append(getStats, stats)
			var v string = ""
			for c:=0; c<len(res);c++ {
				kvp := res[c]
				v = fmt.Sprintf("%s %s", v, kvp.Key)
			}
			log.Println("Successfull mkGET ", v, stats, " ms")
		}
	}

	log.Println(len(setStats), len(getStats))
	sMin, sMax, sAvg, sSd, sPercentiles := cli.GetStats(setStats)
	log.Println("SET stats ", sMin, sMax, sAvg, sSd, sPercentiles)
	gMin, gMax, gAvg, gSd, gPercentiles := cli.GetStats(getStats)
	log.Println("GET stats ", gMin, gMax, gAvg, gSd, gPercentiles)
	return
}