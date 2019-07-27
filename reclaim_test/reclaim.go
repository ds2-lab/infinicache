package main

import (
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"time"
)

var (
	t     = time.Now().String()
	myMap = make(map[string]string)
)

func HandleRequest() {
	myMap["a"] = t
	fmt.Println("Time now is ", time.Now().String(), "Global TimeStamp is ", t, "Stored TimeStamp in Map is", myMap["a"])
}

func main() {
	lambda.Start(HandleRequest)
}
