package main

import (
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"
)

var(
	requestsNumber int
	size           int
	nClients       int
	addrList       string
	startTime      time.Time
	counter        int
	mu             sync.Mutex
)

func main() {
	requestsNumber, size, nClients, addrList = client.GetArgsConcurrent(os.Args)
	addrArr := strings.Split(addrList, ",")
	var wg sync.WaitGroup

	var clients []*client.Client
	for i:=0;i<nClients;i++{
		// initial new ecRedis client
		client := client.NewClient(10, 2, 32, 3)
		// start dial and PUT/GET
		client.Dial(addrArr)
		clients = append(clients, client)
	}

	c := clients[0]
	var data [][3]client.KVSetGroup
	var hKeys []string
	for i:=0;i<1000;i++{
		d := c.GenerateSetData(size)
		hKey := fmt.Sprintf("Hk.%d",i)
		if _, _, ok := c.MkSet(hKey, d); !ok {
			log.Println("Failed to mkSET", hKey)
		}else{
			data = append(data, c.RemoveBytes(d))
			hKeys = append(hKeys, hKey)
			log.Println("Successfull mkSET", hKey)
		}
	}

	counter = nClients

	for i:=0; i< nClients; i++{
		wg.Add(1)
		go testMkConcurrent2(i, clients[i], &wg, data, hKeys)
	}
	wg.Wait()
}

func testMkConcurrent2(i int, cli *client.Client, wg *sync.WaitGroup,
						data [][3]client.KVSetGroup,hKeys []string){
	defer wg.Done()

	mu.Lock()
	if counter == nClients {
		startTime = time.Now()
	}
	counter--
	mu.Unlock()

	for k:=0; k<requestsNumber/nClients; k++{
		random := rand.Intn(len(data))
		d := cli.GenerateSingleRandomGet(data[random])

		if _, _, ok := cli.MkGet(hKeys[random], d); !ok {
			log.Println("Failed to mkGET client ", k, " ", i, " - ", hKeys[random])
		}else{
			//log.Println("Successfull mkGET client", k, " ", i, " - ", hKeys[random])
		}
	}

	mu.Lock()
	counter++
	if counter==nClients {
		duration := time.Since(startTime)
		log.Printf("timeElapsed: %v\n", duration.Seconds())
		log.Printf("GB/s: %v\n", 0.1/float32(duration.Seconds()))
	}
	mu.Unlock()
	return
}