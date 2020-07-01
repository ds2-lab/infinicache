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
	var data [][]client.KVSetGroup

	var setStats []float32

	for k:=0; k<200; k++{
		d := cli.GenerateSetData()
		data = append(data, d)
		if _, stats, ok := cli.MkSet("foo", d); !ok {
			log.Fatal("Failed to SET %v", d)
			return
		}else{
			setStats = append(setStats, stats)
			fmt.Println("Successfull SET %v", d)
		}
	}

	fmt.Println("Average SET time: %d", cli.Average(setStats))

}