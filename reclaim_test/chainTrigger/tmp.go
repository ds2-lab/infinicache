package main

import (
	"fmt"
	"log"
	"time"
)

func main() {
	interval := []int{1, 2, 3, 4, 5}

	log.Println("start")

	duration1 := gen(interval)
	t := time.NewTimer(duration1)

	duration2 := time.Duration(30) * time.Second
	t2 := time.NewTimer(duration2)

	for {
		select {
		case <-t.C:
			log.Println("timer1 expired")
			duration1 := gen(interval)
			log.Println("interval is (min)", duration1)
			t.Reset(duration1)
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
