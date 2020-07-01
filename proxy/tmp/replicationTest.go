package main

import (
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
)

func main() {
	var data [3]client.KVSetGroup
	var g1 = []client.KeyValuePair{
		{Key: "k1", Value: []byte("v1")},
		{Key: "k2", Value: []byte("v2")},
		{Key: "k3", Value: []byte("v3")},
	}
	var g2 = []client.KeyValuePair{
		{Key: "k4", Value: []byte("v4")},
		{Key: "k5", Value: []byte("v5")},
		{Key: "k6", Value: []byte("v6")},
	}
	var g3 = []client.KeyValuePair{
		{Key: "k7", Value: []byte("v7")},
		{Key: "k8", Value: []byte("v8")},
		{Key: "k9", Value: []byte("v9")},
	}

	data[0] = client.KVSetGroup{
		KeyValuePairs: g1,
	}

	data[1] = client.KVSetGroup{
		KeyValuePairs: g2,
	}

	data[2] = client.KVSetGroup{
		KeyValuePairs: g3,
	}

	fmt.Println(replicate(data, 5))
}

func replicate(groups [3]client.KVSetGroup, n int) []client.KVSetGroup {
	var replicas = make([]client.KVSetGroup, n)
	var rFs = []int{5,4,3}
	for i := 0; i < len(groups); i++ {
		var group = groups[i]
		for k := 0; k < len(group.KeyValuePairs); k++{
			for j := 0; j < rFs[i]; j++ {
				replicas[j].KeyValuePairs = append(replicas[j].KeyValuePairs, group.KeyValuePairs[k])
			}
		}
	}
	return replicas
}
