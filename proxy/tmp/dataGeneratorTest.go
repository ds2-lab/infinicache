package main

import (
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"strconv"
)

var i = 1
var j = 1

func main() {
	for i<30 {
		fmt.Println(generateSetData())
		fmt.Println("\n")
	}
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