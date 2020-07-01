package main

import (
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"log"
	"strconv"
	"strings"
)

var I = 1
var J = 1

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
	J = I
	I = I +9
	for ; J <= I; J++ {
		pair := client.KeyValuePair{Key: "k"+strconv.Itoa(J), Value: []byte("v"+string(J))}
		g.KeyValuePairs = append(g.KeyValuePairs, pair)
		if J%3 == 0 && J != 0 {
			data = append(data, g)
			var newG client.KVSetGroup
			g = newG
		}
	}
	return data
}

func average(xs[]float32)float32 {
	total:=float32(0)
	for _,v:=range xs {
		total +=v
	}
	return total/float32(len(xs))
}

//func generateRandomGet(data [][]client.KVSetGroup) [][]client.KVGetGroup{
//	var output [][]client.KVGetGroup
//	for i:=0; i<len(data); i++{
//		d := data[i]
//		query =
//		for j:=0; j<len(d); j++{
//			g := d[j]
//			randomSelect := rand.Intn(len(g.KeyValuePairs))
//		}
//
//	}
//	return output
//}