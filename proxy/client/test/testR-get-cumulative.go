package main

import (
	"bytes"
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"log"
	"math/rand"
	"os"
	"strings"
)

func main() {
	requestsNumber, size, addrList := client.GetArgs(os.Args)
	val := make([]byte, size)
	rand.Read(val)

	// parse server address
	addrArr := strings.Split(addrList, ",")

	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32, 3)

	// start dial and PUT/GET
	cli.Dial(addrArr)
	var getStats []float64

	for k:=0; k<=requestsNumber; k++{
		key := fmt.Sprintf("k.%d", k)

		var s float64 = 0
		for l:=0; l<3; l++ {
			key := fmt.Sprintf("%s.%d", key, l)
			if _, reader, stats, ok := cli.RGet(key, len(val)); !ok {
				log.Println("Failed to get ", key)
				return
			} else {
				buf := new(bytes.Buffer)
				buf.ReadFrom(reader)
				reader.Close()
				//s := buf.String()
				log.Println("Successful rGET", key)
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

