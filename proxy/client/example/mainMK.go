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


	// parse server address
	addrArr := strings.Split(addrList, ",")

	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32, 3)

	// start dial and PUT/GET
	cli.Dial(addrArr)
	if _, ok := cli.MkSet("foo", data); !ok {
		log.Fatal("Failed to set")
		return
	}else{
		fmt.Println("successfull SET")
	}

	var keys [3]client.KVGetGroup
	keys[0] = client.KVGetGroup{Keys: []string{"k1", "k2"}}
	keys[1] = client.KVGetGroup{Keys: []string{"k4"}}
	keys[2] = client.KVGetGroup{Keys: []string{"k5"}}

	fmt.Println(cli.MkGet("foo", keys))
	return
}