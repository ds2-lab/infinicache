package main

import (
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	lambdaService "github.com/aws/aws-sdk-go/service/lambda"
)

func handleReq() {
	fmt.Println("hello world this is ", lambdacontext.FunctionName)
	trigg("test2")
	fmt.Println("trigger complete")
}

func trigg(name string) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	client := lambdaService.New(sess, &aws.Config{Region: aws.String("us-east-1")})
	fmt.Println("before Invoke Input")
	input := &lambdaService.InvokeInput{
		FunctionName:   aws.String(name),
		InvocationType: aws.String("Event"),
	}
	res, err := client.Invoke(input)
	fmt.Println("res", *res.StatusCode)
	if err != nil {
		fmt.Println("Error calling migration lambda %s, %v", name, err)
	}
}
func main() {
	lambda.Start(handleReq)
}
