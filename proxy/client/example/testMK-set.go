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
	var addrList = "10.4.0.100:6378"
	// initial object with random value

	requestsNumber, _ := strconv.Atoi(os.Args[0])

	// parse server address
	addrArr := strings.Split(addrList, ",")

	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32, 3)

	// start dial and PUT/GET
	cli.Dial(addrArr)
	var data [][3]client.KVSetGroup

	var setStats []float64

	for k:=0; k<requestsNumber; k++{
		d := cli.GenerateSetData(1313)
		data = append(data, d)
		key := fmt.Sprintf("HighLevelKey-%d", k)
		if _, stats, ok := cli.MkSet(key, d); !ok {
			log.Println("Failed to mkSET", key)
		}else{
			setStats = append(setStats, stats)
			log.Println("Successfull mkSET", key)
		}
	}

	fmt.Println("Average mkSET time: ", cli.Average(setStats))

}