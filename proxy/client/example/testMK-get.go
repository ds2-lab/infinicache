package main

import (
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"log"
	"os"
	"strconv"
	"strings"
)

func main() {
	var addrList = "10.4.0.100:6378,10.4.14.71:6378"
	// initial object with random value
	requestsNumber, err := strconv.Atoi(os.Args[1])
	if err!=nil{
		log.Fatal("No arguments for test. requests number expected")
	}
	// parse server address
	addrArr := strings.Split(addrList, ",")

	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32, 3)

	// start dial and PUT/GET
	cli.Dial(addrArr)

	var getStats []float64

	for k:=0; k<requestsNumber; k++{
		d_in := cli.GenerateSetData(1313)
		d_out := cli.GenerateSingleRandomGet(d_in)
		key := fmt.Sprintf("HighLevelKey-%d", k)
		if res, stats, ok := cli.MkGet(key, d_out); !ok {
			log.Println("Failed to mkGET %v", d_out)
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


	fmt.Println("Average mkGET time: ", cli.Average(getStats))

}