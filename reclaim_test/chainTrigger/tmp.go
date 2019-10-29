package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"
)

func gen(min int, max int) time.Duration {
	rand.Seed(time.Now().UnixNano())
	n := min + rand.Intn(max-1+min)
	duration := time.Duration(n) * time.Second
	return duration
}
func main() {
	log.Println("start")

	duration1 := gen(1, 5)
	t := time.NewTimer(duration1)
	duration2 := time.Duration(30) * time.Second
	t2 := time.NewTimer(duration2)

	for {
		select {
		case <-t.C:
			log.Println("timer1 expired")
			duration1 := gen(1, 5)
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
