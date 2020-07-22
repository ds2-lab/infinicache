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
	var setStats []float64
	var getStats []float64

	for k:=0; k<requestsNumber; k++{
		key := "foo" + strconv.Itoa(k)
		if _, stats, ok := cli.RSet(key, val); !ok {
			log.Println("Failed to set ", key)
		}else{
			log.Println("Succesfully rSET", key)
			setStats = append(setStats, stats)
		}
	}

	for k:=0; k<len(setStats); k++{
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

	log.Println(len(setStats), len(getStats))
	sMin, sMax, sAvg, sSd, sPercentiles := cli.GetStats(setStats)
	log.Println("SET stats ", sMin, sMax, sAvg, sSd, sPercentiles)
	gMin, gMax, gAvg, gSd, gPercentiles := cli.GetStats(getStats)
	log.Println("GET stats ", gMin, gMax, gAvg, gSd, gPercentiles)
	return
}

