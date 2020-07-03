package main

import (
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"log"
	"strings"
)

func main() {
	var addrList = "10.4.0.100:6378"
	// initial object with random value

	// parse server address
	addrArr := strings.Split(addrList, ",")

	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32, 3)

	// start dial and PUT/GET
	cli.Dial(addrArr)
	var data [][3]client.KVSetGroup

	var setStats []float64
	var getStats []float64

	getData := cli.GenerateRandomGet(data)

	for k:=0; k<len(getData); k++{
		d := getData[k]
		key := fmt.Sprintf("HighLevelKey-%d", k)
		if res, stats, ok := cli.MkGet(key, d); !ok {
			log.Println("Failed to mkGET %v", d)
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

	fmt.Println("Average mkSET time: ", cli.Average(setStats))
	fmt.Println("Average mkGET time: ", cli.Average(getStats))

}