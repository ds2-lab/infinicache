package main

import (
	"bytes"
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
		go test3(i, &wg, addrList, requestsNumber, size)
	}
	wg.Wait()
}

func test3(i int, wg *sync.WaitGroup, addrList string, reqNumber int, size int){
	defer wg.Done()

	// parse server address
	addrArr := strings.Split(addrList, ",")

	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32, 3)

	// start dial and PUT/GET
	cli.Dial(addrArr)

	// ToDo: Replace with a channel
	var getStats []float64
	val := make([]byte, size)
	rand.Read(val)

	for k:=0; k<reqNumber; k++{

		key := fmt.Sprintf("%d.k.%d",i, k)
		if _, reader, stats, ok := cli.RGet(key, len(val)); !ok {
			log.Println("Failed to get ", key)
			return
		} else {
			buf := new(bytes.Buffer)
			buf.ReadFrom(reader)
			reader.Close()
			//s := buf.String()
			log.Println("Successfull rGET", key)
			getStats = append(getStats, stats)
		}
	}

	gMin, gMax, gAvg, gSd, gPercentiles := cli.GetStats(getStats)
	log.Println("GET stats ", gMin, gMax, gAvg, gSd, gPercentiles)
	return
}



