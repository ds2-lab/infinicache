package main

import (
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"log"
	"os"
	"strings"
	"sync"
)

func main() {
	requestsNumber, size, addrList := client.GetArgs(os.Args)
	var wg sync.WaitGroup
	for i:=0; i<3; i++{
		wg.Add(1)
		go test(i, &wg, addrList, requestsNumber, size)
	}
	wg.Wait()
}

func test(i int, wg *sync.WaitGroup, addrList string, reqNumber int, size int){
	defer wg.Done()

	// parse server address
	addrArr := strings.Split(addrList, ",")

	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32, 3)

	// start dial and PUT/GET
	cli.Dial(addrArr)
	var data [][3]client.KVSetGroup

	// ToDo: Replace with a channel
	var setStats []float64
	//var getStats []float64

	for k:=0; k<reqNumber; k++{
		d := cli.GenerateSetData(size)
		data = append(data, d)
		key := fmt.Sprintf("%d.Hk.%d",i, k)
		if _, stats, ok := cli.MkSet(key, d); !ok {
			log.Println("Failed to mkSET", i, key)
		}else{
			setStats = append(setStats, stats)
			log.Println("Successfull mkSET", i, key)
		}
	}

	//getData := cli.GenerateRandomGet(data)
	//
	//for k:=0; k<len(getData); k++{
	//	d := getData[k]
	//	key := fmt.Sprintf("HighLevelKey-%d", k)
	//	if res.csv, stats, ok := cli.MkGet(key, d); !ok {
	//		log.Println("Failed to mkGET ", i, " ", key)
	//	}else{
	//		getStats = append(getStats, stats)
	//		var v string = ""
	//		for c:=0; c<len(res.csv);c++ {
	//			kvp := res.csv[c]
	//			v = fmt.Sprintf("%s %s", v, kvp.Key)
	//		}
	//		log.Println("Successfull mkGET ",i, " ", v, stats, " ms")
	//	}
	//}

	sMin, sMax, sAvg, sSd, sPercentiles := cli.GetStats(setStats)
	log.Println("SET stats ", sMin, sMax, sAvg, sSd, sPercentiles)
	return
}