package main

import (
	"fmt"
	"strings"
)

func main() {
	a := fmt.Sprintf("%s,%s,%s,%s", "func1","123","func1","234")

	fmt.Println(a)

	s := strings.Split(a,",")
	fmt.Println(s)

}

func temp() string {
	a := fmt.Sprintf("%d\n%d\n", 1, 2)
	return a
}
