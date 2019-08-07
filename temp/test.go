package main

import (
	"fmt"
	"sync"
	"time"
)

func main() {
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go tmp(i, &wg)
	}
	wg.Wait()
	var count int32
	count = 10
	fmt.Println(fmt.Sprint(count) + "1")
}

func tmp(i int, wg *sync.WaitGroup) {
	time.Sleep(2 * time.Second)
	fmt.Println("this is ", i)
	wg.Done()
}
