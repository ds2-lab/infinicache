package main

import (
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"log"
	"strconv"
	"strings"
	"time"
)

var i = 1
var j = 1

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

	var setStats []int64

	for k:=0; k<200; k++{
		d := generateSetData()
		data = append(data, d)
		if _, stats, ok := cli.MkSet("foo", d); !ok {
			log.Fatal("Failed to SET %v", d)
			return
		}else{
			setStats = append(setStats, stats)
			fmt.Println("Successfull SET %v", d)
		}
	}

	fmt.Println("Average SET time: %d", average(setStats))


}

func generateSetData() []client.KVSetGroup{
	var data []client.KVSetGroup
	var g client.KVSetGroup
	j = i
	i = i+9
	for ;j<=i;j++ {
		pair := client.KeyValuePair{Key: "k"+strconv.Itoa(j), Value: []byte("v"+string(j))}
		g.KeyValuePairs = append(g.KeyValuePairs, pair)
		if j%3 == 0 && j!= 0 {
			data = append(data, g)
			var newG client.KVSetGroup
			g = newG
		}
	}
	return data
}

func average(xs[]int64)int64 {
	total:=int64(0)
	for _,v:=range xs {
		total +=v
	}
	return total/int64(len(xs))
}

func toMillisec(m int64) time.Time {
	return time.Unix(0, m * int64(time.Millisecond))
}