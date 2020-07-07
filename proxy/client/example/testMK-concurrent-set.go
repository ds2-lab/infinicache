package main

import (
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
)

func main() {
	var wg sync.WaitGroup
	requestsNumber, _ := strconv.Atoi(os.Args[1])
	for i:=0; i<3; i++{
		wg.Add(1)
		go test(i, &wg, requestsNumber)
	}
	wg.Wait()
}

func test(i int, wg *sync.WaitGroup, requestsNumber int){
	defer wg.Done()
	var addrList = "10.4.0.100:6378,10.4.14.71:6378"
	// initial object with random value

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

	for k:=0; k<requestsNumber; k++{
		d := cli.GenerateSetData(5)
		data = append(data, d)
		key := fmt.Sprintf("HighLevelKey-%d", k)
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
	//	if res, stats, ok := cli.MkGet(key, d); !ok {
	//		log.Println("Failed to mkGET ", i, " ", key)
	//	}else{
	//		getStats = append(getStats, stats)
	//		var v string = ""
	//		for c:=0; c<len(res);c++ {
	//			kvp := res[c]
	//			v = fmt.Sprintf("%s %s", v, kvp.Key)
	//		}
	//		log.Println("Successfull mkGET ",i, " ", v, stats, " ms")
	//	}
	//}

	log.Println(cli.Average(setStats))
}