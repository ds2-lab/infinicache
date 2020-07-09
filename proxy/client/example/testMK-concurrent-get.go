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
		go test2(i, &wg, addrList, requestsNumber, size)
	}
	wg.Wait()
}

func test2(i int, wg *sync.WaitGroup, addrList string, reqNumber int, size int){
	defer wg.Done()

	// parse server address
	addrArr := strings.Split(addrList, ",")

	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32, 3)

	// start dial and PUT/GET
	cli.Dial(addrArr)

	// ToDo: Replace with a channel
	var getStats []float64

	for k:=0; k<reqNumber; k++{
		sD := cli.GenerateSetData(size)
		d := cli.GenerateSingleRandomGet(sD)
		key := fmt.Sprintf("%d.Hk.%d",i, k)

		if res, stats, ok := cli.MkGet(key, d); !ok {
			log.Println("Failed to mkGET ", i, " ", key)
		}else{
			getStats = append(getStats, stats)
			var v string = ""
			for c:=0; c<len(res);c++ {
				kvp := res[c]
				v = fmt.Sprintf("%s %s", v, kvp.Key)
			}
			log.Println("Successfull mkGET ",i, " ", v, stats, " ms")
		}
	}

	gMin, gMax, gAvg, gSd, gPercentiles := cli.GetStats(getStats)
	log.Println("GET stats ", gMin, gMax, gAvg, gSd, gPercentiles)
	return
}