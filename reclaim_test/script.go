package main

import (
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"time"
)

type lambdaInstance struct {
	name  string
	alive bool
}

var (
	n = flag.String("name", "reclaimTest", "lambda function name")
)

func main() {
	flag.Parse()
	l := newLambdaInstance(*n)
	t := time.NewTicker(10 * time.Minute)
	for {
		select {
		case <-t.C:
			lambdaTrigger(l)
		}
	}
}

func newLambdaInstance(name string) *lambdaInstance {
	return &lambdaInstance{
		name:  name,
		alive: false,
	}
}

func lambdaTrigger(l *lambdaInstance) {
	l.alive = true
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	client := lambda.New(sess, &aws.Config{Region: aws.String("us-east-1")})

	_, err := client.Invoke(&lambda.InvokeInput{FunctionName: aws.String(l.name)})
	if err != nil {
		fmt.Println("Error calling LambdaFunction", err)
	}

	fmt.Println("Lambda Deactivate")
	l.alive = false
}
