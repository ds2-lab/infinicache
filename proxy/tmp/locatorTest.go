package main

import (
	"fmt"
	"github.com/fatih/set"
	"github.com/neboduus/infinicache/proxy/client"
	"math/rand"
	"strings"
)

func main() {

	var addrList = "10.4.0.100:6378"

	// parse server address
	addrArr := strings.Split(addrList, ",")

	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32, 3)
	cli.Dial(addrArr)
	var data [][3]client.KVSetGroup

	d := cli.GenerateSetData()
	data = append(data, d)

	gData := cli.GenerateRandomGet(data)
	fmt.Println(locateLowLevelKeys(gData[0]))
}

func locateLowLevelKeys(groups [3]client.KVGetGroup) map[int]set.Interface {
	var replicas [3]int
	var m map[int]set.Interface

	fmt.Println(groups)

	for i:=0; i<len(groups); i++{
		g := groups[i]
		if g.Keys != nil {
			replicas[i] = rand.Intn(len(groups)+i)
		}else{
			replicas[i] = -1
		}
	}
	m = make(map[int]set.Interface)
	for i:=0; i<len(replicas); i++ {
		if replicas[i] >= 0 {
			if m[replicas[i]] == nil {
				m[replicas[i]] = set.New(set.ThreadSafe)
			}
			for k:=0; k<len(groups[i].Keys); k++ {
				m[replicas[i]].Add(groups[i].Keys[k])
			}
		}
	}

	return m
}