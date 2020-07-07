package main

import (
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"log"
	"strings"
	"time"
)

func main() {

	var addrList = "10.4.0.100:6378,10.4.14.71:6378"
	// initial object with random value

	// parse server address
	addrArr := strings.Split(addrList, ",")

	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32, 3)

	// start dial and PUT/GET
	cli.Dial(addrArr)
	var data [][3]client.KVSetGroup

	var setStats []float64

	ticker := time.NewTicker(1 * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <- ticker.C:
				for k:=0; k<3; k++{
					d := cli.GenerateSetData(5)
					data = append(data, d)
					key := fmt.Sprintf("HighLevelKey-%d", k)
					if _, stats, ok := cli.MkSet(key, d); !ok {
						log.Println("Failed to mkSET", key)
					}else{
						setStats = append(setStats, stats)
						log.Println("Successfull mkSET", key)
					}
				}
			case <- quit:
				ticker.Stop()
				return
			}
			log.Println(cli.Average(setStats))
		}
	}()

}