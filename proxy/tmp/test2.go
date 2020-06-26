package main

import (
	"fmt"
	"github.com/fatih/set"
	"github.com/neboduus/infinicache/proxy/client"
	"math/rand"
)

func main() {
	var data [3]client.KVGetGroup
	var g1 = []string{"k1", "k2", "k3"}
	var g2 = []string{"k4", "k5", "k6"}
	var g3 = []string{"k7", "k8", "k9"}


	data[0] = client.KVGetGroup{
		Keys: g1,
	}

	data[1] = client.KVGetGroup{
		Keys: g2,
	}

	data[2] = client.KVGetGroup{
		Keys: g3,
	}

	m := locateLowLevelKeys(data)
	fmt.Println(m)
	fmt.Println(len(m))

	s := set.New(set.ThreadSafe)
	s.Add("ciaoo")
	lowLevelKeysList := s.List()
	for i := 0; i < len(lowLevelKeysList); i++ {
		fmt.Println(lowLevelKeysList[i].(string))
	}
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