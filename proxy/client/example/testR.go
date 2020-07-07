package main

import (
	"bytes"
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"log"
	"os"
	"strconv"
	"strings"
)

func main() {
	var addrList = "10.4.0.100:6378"
	// initial object with random value
	requestsNumber, err := strconv.Atoi(os.Args[0])
	if err!=nil{
		log.Fatal("No arguments for test. requests number expected")
	}
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
	var getStats []float64

	for k:=0; k<requestsNumber; k++{
		key := "foo" + strconv.Itoa(k)
		if _, stats, ok := cli.RSet(key, val); !ok {
			log.Println("Failed to set ", key)
		}else{
			log.Println("Succesfully rSET", key)
			setStats = append(setStats, stats)
		}
	}

	for k:=0; k<len(setStats); k++{
		key := "foo" + strconv.Itoa(k)
		if _, reader, stats, ok := cli.RGet(key, len(val)); !ok {
			log.Println("Failed to get ", key)
			return
		} else {
			buf := new(bytes.Buffer)
			buf.ReadFrom(reader)
			reader.Close()
			//s := buf.String()
			log.Println("Successfull rGET", key)
			getStats = append(getStats, stats)
		}
	}

	log.Println("Average mkSET time: ", cli.Average(setStats))
	log.Println("Average mkGET time: ", cli.Average(getStats))


}

