package main

import (
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"math/rand"
	"strconv"
)

var M = 1
var N = 1

func main() {
	a := generateSetData()
	var array [][3]client.KVSetGroup
	array = append(array, a)
	fmt.Println(a)
	fmt.Println(generateRandomGet(array))
}

func generateSetData() [3]client.KVSetGroup{
	var data [3]client.KVSetGroup
	var g client.KVSetGroup
	N = M
	M = M +9
	k := 0
	for ; N <= M; N++ {
		pair := client.KeyValuePair{Key: "k"+strconv.Itoa(N), Value: []byte("v"+string(N))}
		g.KeyValuePairs = append(g.KeyValuePairs, pair)
		if N%3 == 0 && N != 0 {
			data[k] = g
			k++
			var newG client.KVSetGroup
			g = newG
		}
	}
	return data
}

func generateRandomGet(data [][3]client.KVSetGroup) [][3]client.KVGetGroup{
	var output [][3]client.KVGetGroup

	for i:=0; i<len(data); i++{
		d := data[i]
		var getGroups [3]client.KVGetGroup

		for j:=0; j<len(d); j++{
			g := d[j]
			var getG client.KVGetGroup
			query := []string{}
			query = append(query, g.KeyValuePairs[rand.Intn(len(g.KeyValuePairs))].Key)
			getG.Keys = query
			getGroups[j] = getG
		}

		output = append(output, getGroups)

	}
	return output
}