//Chain trigger version
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	lambdaService "github.com/aws/aws-sdk-go/service/lambda"
	protocol "github.com/wangaoone/LambdaObjectstore/reclaim_test/types"
	"sync"
	"time"
)

var (
	t = time.Now().UnixNano()
	//hostName     string
	srcCount = 0
	//replicaCount = 0
)

//func init() {
//	cmd := exec.Command("uname", "-a")
//	host, err := cmd.CombinedOutput()
//	if err != nil {
//		fmt.Println("cmd.Run() failed with ", err)
//	}
//	hostName = strings.Split(string(host), " #")[0]
//}
//func HandleRequest(ctx context.Context, input protocol.InputEvent) (string, error) {
//	//// get first lambda info
//	//res := fmt.Sprintf("%s,%s,%d", lambdacontext.FunctionName, hostName, t)
//	//if input.Cmd == "trigger" {
//	//	fmt.Println("going to trigger mySelf")
//	//	output := trigger(lambdacontext.FunctionName)
//	//	// get replica lambda info
//	//	res = fmt.Sprintf("%s,%s", res, output)
//	//	fmt.Println("me and my replica is", res)
//	//	return res, nil
//	//}
//	switch input.Cmd {
//	case "backup":
//		srcCount = 0
//		replicaRes := fmt.Sprintf("%s,%d", lambdacontext.FunctionName, t)
//		//srcCount = srcCount + 1
//		return replicaRes, nil
//	default:
//		srcRes := fmt.Sprintf("%s,%d", lambdacontext.FunctionName, t)
//		if srcCount%5 == 0 && srcCount != 0 {
//			output := trigger(lambdacontext.FunctionName)
//			res := fmt.Sprintf("%s\n%s", srcRes, output)
//			srcCount = srcCount + 1
//			return res, nil
//		}
//		srcCount = srcCount + 1
//		return srcRes, nil
//	}
//}
func HandleRequest(ctx context.Context, input protocol.InputEvent) (string, error) {
	switch input.Cmd {
	case "backup1":
		fmt.Println("this is destination lambda with backup1")
		replicaRes := fmt.Sprintf("%s,%d", lambdacontext.FunctionName, t)
		return replicaRes, nil
	case "backup2":
		fmt.Println("this is destination lambda with backup2")
		replicaRes := fmt.Sprintf("%s,%d", lambdacontext.FunctionName, t)
		return replicaRes, nil
	default:
		fmt.Println("this is source lambda")
		srcRes := fmt.Sprintf("%s,%d", lambdacontext.FunctionName, t)
		output := trigger(lambdacontext.FunctionName)
		result := fmt.Sprintf("%s,%s", srcRes, output)
		return result, nil
	}
}

func main() {
	lambda.Start(HandleRequest)
}

func input(name string, cmd string) *lambdaService.InvokeInput {
	event1 := &protocol.InputEvent{
		Cmd: cmd,
	}
	payload1, _ := json.Marshal(event1)
	input := &lambdaService.InvokeInput{
		FunctionName: aws.String(name),
		Payload:      payload1,
	}
	return input
}

func trigger(name string) string {
	var res1 string
	var res2 string
	var wg sync.WaitGroup

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	client := lambdaService.New(sess, &aws.Config{Region: aws.String("us-east-1")})
	i1 := input(name, "backup1")
	i2 := input(name, "backup2")
	wg.Add(2)
	go func() {
		output1, err := client.Invoke(i1)
		if err != nil {
			fmt.Println("Error calling LambdaFunction", err)
		}
		res1 = string(output1.Payload)[1 : len(string(output1.Payload))-1]
		wg.Done()
	}()

	go func() {
		output2, err := client.Invoke(i2)
		if err != nil {
			fmt.Println("Error calling LambdaFunction", err)
		}
		res2 = string(output2.Payload)[1 : len(string(output2.Payload))-1]
		wg.Done()
	}()
	wg.Wait()
	return fmt.Sprintf("%s,%s", res1, res2)
}
