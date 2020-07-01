package main

import (
	"bytes"
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"log"
	"strconv"
	"strings"
)

func main() {
	var addrList = "10.4.0.100:6378"
	// initial object with random value
	var val []byte
	val = []byte("test")

	// parse server address
	addrArr := strings.Split(addrList, ",")

	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32, 3)

	// start dial and PUT/GET
	cli.Dial(addrArr)
	var setStats []float32
	var getStats []float32

	for k:=0; k<1000; k++{
		key := "foo" + strconv.Itoa(k)
		if _, stats, ok := cli.RSet(key, val); !ok {
			log.Fatal("Failed to set ", key)
			return
		}else{
			fmt.Println("Succesfully Set")
			setStats = append(setStats, stats)
		}

		if _, reader, stats, ok := cli.RGet(key, len(val)); !ok {
			log.Fatal("Failed to get ", key)
			return
		} else {
			buf := new(bytes.Buffer)
			buf.ReadFrom(reader)
			reader.Close()
			s := buf.String()
			fmt.Println("received value: ", s)
			getStats = append(getStats, stats)
		}
	}

	fmt.Println("Average mkSET time: ", cli.Average(setStats))
	fmt.Println("Average mkGET time: ", cli.Average(getStats))


}

