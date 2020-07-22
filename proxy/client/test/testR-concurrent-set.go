package main

import (
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
)

func main() {
	requestsNumber, size, addrList := client.GetArgs(os.Args)
	var wg sync.WaitGroup
	for i:=0; i<3; i++{
		wg.Add(1)
		go test4(i, &wg, addrList, requestsNumber, size)
	}
	wg.Wait()
}

func test4(i int, wg *sync.WaitGroup, addrList string, reqNumber int, size int){
	defer wg.Done()

	// parse server address
	addrArr := strings.Split(addrList, ",")

	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32, 3)

	// start dial and PUT/GET
	cli.Dial(addrArr)

	// ToDo: Replace with a channel
	var setStats []float64
	val := make([]byte, size)
	rand.Read(val)

	for k:=0; k<reqNumber; k++{

		key := fmt.Sprintf("%d.k.%d",i, k)
		if _, stats, ok := cli.RSet(key, val); !ok {
			log.Println("Failed to set ", key)
		}else{
			log.Println("Succesfully rSET", key)
			setStats = append(setStats, stats)
		}
	}

	gMin, gMax, gAvg, gSd, gPercentiles := cli.GetStats(setStats)
	log.Println("GET stats ", gMin, gMax, gAvg, gSd, gPercentiles)
	return
}



