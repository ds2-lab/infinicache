package main

import (
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"log"
	"os"
	"strings"
)

func main() {
	requestsNumber, size, addrList := client.GetArgs(os.Args)

	// parse server address
	addrArr := strings.Split(addrList, ",")

	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32, 3)

	// start dial and PUT/GET
	cli.Dial(addrArr)

	var getStats []float64

	for k:=0; k<requestsNumber; k++{
		d_in := cli.GenerateSetData(size)
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