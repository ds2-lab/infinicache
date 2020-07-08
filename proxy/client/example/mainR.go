package main

import (
	"bytes"
	"fmt"
	"github.com/neboduus/infinicache/proxy/client"
	"log"
	"os"
	"strings"
	"math/rand"
)

func main() {
	cli := client.NewClient(10, 2, 32, 3)
	_, size, addrList := client.GetArgs(os.Args)
	// initial object with random value
	val := make([]byte, size)
	rand.Read(val)

	// parse server address
	addrArr := strings.Split(addrList, ",")


	// start dial and PUT/GET
	cli.Dial(addrArr)

	key := "foo"
	if _, _, ok := cli.RSet(key, val); !ok {
		log.Fatal("Failed to set ", key)
		return
	}else{
		fmt.Println("Succesfully Set")
	}

	if _, reader, _, ok := cli.RGet(key, len(val)); !ok {
		log.Fatal("Failed to get ", key)
		return
	} else {
		buf := new(bytes.Buffer)
		buf.ReadFrom(reader)
		reader.Close()
		s := buf.String()
		fmt.Println("received value: ", s)
	}


}

