package main

import (
	"bytes"
	"github.com/neboduus/infinicache/proxy/client"
	"log"
	"os"
	"strconv"
	"strings"
	"math/rand"
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
	requestsNumber, err := strconv.Atoi(os.Args[1])
	if err!=nil{
		log.Fatal("No arguments for test. requests number expected")
	}

	for k:=0; k<requestsNumber; k++{
		key := "foo" + strconv.Itoa(k)
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

