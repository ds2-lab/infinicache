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
	var data [3]client.KVGroup
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

	data[0] = client.KVGroup{
		KeyValuePairs: g1,
	}

	data[1] = client.KVGroup{
		KeyValuePairs: g2,
	}

	data[2] = client.KVGroup{
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

	/*if _, reader, ok := cli.RGet("foo", len(val)); !ok {
		log.Fatal("Failed to get")
		return
	} else {
		buf := new(bytes.Buffer)
		buf.ReadFrom(reader)
		reader.Close()
		s := buf.String()
		fmt.Println("received value: ")
		fmt.Println(s)
	}*/
}