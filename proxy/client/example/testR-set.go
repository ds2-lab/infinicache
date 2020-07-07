package main

import (
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"log"
	"os"
	"strconv"
	"strings"
)

func main() {
	var addrList = "10.4.0.100:6378"
	requestsNumber, err := strconv.Atoi(os.Args[0])
	if err!=nil{
		log.Fatal("No arguments for test. requests number expected")
	}
	// initial object with random value
	var val []byte
	var s string = ""
	for k:=0;k<1313;k++{
		s = fmt.Sprintf("v%s", s)
	}
	val = []byte(s)

	// parse server address
	addrArr := strings.Split(addrList, ",")

	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32, 3)

	// start dial and PUT/GET
	cli.Dial(addrArr)
	var setStats []float64

	for k:=0; k<requestsNumber; k++{
		key := "foo" + strconv.Itoa(k)
		if _, stats, ok := cli.RSet(key, val); !ok {
			log.Println("Failed to set ", key)
		}else{
			log.Println("Succesfully rSET", key)
			setStats = append(setStats, stats)
		}
	}

	log.Println("Average rSET time: ", cli.Average(setStats))

}

