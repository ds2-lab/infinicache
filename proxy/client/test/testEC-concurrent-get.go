package main

import (
	"math/rand"
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
		go test5(i, &wg, addrList, requestsNumber, size)
	}
	wg.Wait()
}

func test5(i int, wg *sync.WaitGroup, addrList string, reqNumber int, size int){
	defer wg.Done()

	// parse server address
	addrArr := strings.Split(addrList, ",")

	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32, 3)

	// start dial and PUT/GET
	cli.Dial(addrArr)
	val := make([]byte, size)
	rand.Read(val)
	// ToDo: Replace with a channel
	var getStats []float64

	for k:=0; k<reqNumber; k++{
		key := fmt.Sprintf("k.%d.%d", i, k)

		var s float64 = 0
		for l:=0; l<3; l++ {
			key := fmt.Sprintf("%s.%d", key, l)
			if _, _, stats, ok := cli.EcGet(key, size); !ok {
				log.Println("Failed to GET ", key)
			} else {
				log.Println("Successfull GET ", key)
				getStats = append(getStats, stats)
			}
		}
		if s!=0{
			getStats = append(getStats, s)
		}
	}

	gMin, gMax, gAvg, gSd, gPercentiles := cli.GetStats(getStats)
	log.Println("SET stats ", gMin, gMax, gAvg, gSd, gPercentiles)
	return
}