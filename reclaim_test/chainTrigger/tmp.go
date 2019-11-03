package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"
)

func temp_gen(interval []int) time.Duration {
	rand.Seed(time.Now().UnixNano())
	duration := time.Duration(interval[rand.Intn(len(interval))]) * time.Second
	return duration
}

func main() {
	interval := []int{5, 8, 10}

	log.Println("start")

	//duration1 := temp_gen(interval)
	duration1 := time.Duration(20) * time.Second
	t := time.NewTimer(duration1)

	duration2 := time.Duration(60) * time.Second
	t2 := time.NewTimer(duration2)

	duration3 := temp_gen(interval)
	t3 := time.NewTimer(duration3)

	for {
		select {
		case <-t.C:
			log.Println("trigger src and replica, timer1 expired")
			t.Reset(duration1)
		case <-t3.C:
			log.Println("only trigger src, timer3 expired")
			duration3 = temp_gen(interval)
			log.Println("interval is (min)", duration3)
			t3.Reset(duration3)
		case <-t2.C:
			log.Println("warm up finished")
			return

		}
	}
}

func temp() string {
	a := fmt.Sprintf("%d\n%d\n", 1, 2)
	return a
}
