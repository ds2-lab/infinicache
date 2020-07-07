package main

import (
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
)

func main() {
	addrList := "10.4.0.100:6378,10.4.0.100:6378"
	requestsNumber, err := strconv.Atoi(os.Args[0])
	if err!=nil{
		log.Fatal("No arguments for test. requests number expected")
	}
	s := ""
	for k:=0;k<1313;k++{
		s = fmt.Sprintf("v%s", s)
	}
	val := []byte(s)

	// parse server address
	addrArr := strings.Split(addrList, ",")

	// initial new ecRedis client
	cli := client.NewClient(10, 2, 32, 3)
	var setStats []float64
	//var getStats []float64
	cli.Dial(addrArr)



	for k:=0; k<=requestsNumber; k++{
		key := fmt.Sprintf("k%d", k)

		s := [9]float64{0,0,0,0,0,0,0,0,0}
		var wg sync.WaitGroup
		for l:=0; l<9; l++ {

			key := fmt.Sprintf("%s%d", key, l)
			wg.Add(1)
			go concurrentSet(cli, key, val, &wg, s, l)

		}
		wg.Wait()
		for l:=0; l<9; l++{
			if l != 0 {
				setStats = append(setStats, s[l])
			}
		}

	}

/*	for k:=0; k<=1000; k++{
		key := fmt.Sprintf("k%d", k)

		var s float64 = 0
		for l:=0; l<3; l++ {
			key := fmt.Sprintf("%s%d", key, l)
			if _, _, stats, ok := cli.EcGet(key, 160); !ok {
				log.Println("Failed to GET ", key)
			} else {
				log.Println("Successfull GET ", key)
				s += stats
			}
		}
		if s != 0{
			getStats = append(getStats, s)
		}

	}*/
	log.Println("Average SET time: ", cli.Average(setStats))
	//log.Println("Average GET time: ", cli.Average(getStats))
	return
}

func concurrentSet(cli *client.Client, key string, val []byte, waitGroup *sync.WaitGroup, stats [9]float64, i int){
	defer waitGroup.Done()

	if _, s, ok := cli.EcSet(key, val); !ok {
		log.Println("Failed to SET ", key)
	}else{
		log.Println("Successfull SET ", key)
		stats[i] = s
	}
}