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
func HandleRequest(ctx context.Context, input protocol.InputEvent) (string, error) {
	//// get first lambda info
	//res := fmt.Sprintf("%s,%s,%d", lambdacontext.FunctionName, hostName, t)
	//if input.Cmd == "trigger" {
	//	fmt.Println("going to trigger mySelf")
	//	output := trigger(lambdacontext.FunctionName)
	//	// get replica lambda info
	//	res = fmt.Sprintf("%s,%s", res, output)
	//	fmt.Println("me and my replica is", res)
	//	return res, nil
	//}
	switch input.Cmd {
	case "backup":
		replicaRes := fmt.Sprintf("%d,%s,%d", srcCount, lambdacontext.FunctionName, t)
		//srcCount = srcCount + 1
		return replicaRes, nil
	default:
		srcRes := fmt.Sprintf("%d,%s,%d", srcCount, lambdacontext.FunctionName, t)
		if srcCount%5 == 0 && srcCount != 0 {
			output := trigger(lambdacontext.FunctionName)
			res := fmt.Sprintf("%s,%s", srcRes, output)
			srcCount = srcCount + 1
			return res, nil
		}
		srcCount = srcCount + 1
		return srcRes, nil
	}
}

func main() {
	lambda.Start(HandleRequest)
}

func trigger(name string) string {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	client := lambdaService.New(sess, &aws.Config{Region: aws.String("us-east-1")})
	event := &protocol.InputEvent{
		Cmd: "backup",
	}
	payload, _ := json.Marshal(event)
	input := &lambdaService.InvokeInput{
		FunctionName: aws.String(name),
		Payload:      payload,
	}
	output, err := client.Invoke(input)
	if err != nil {
		fmt.Println("Error calling LambdaFunction", err)
	}
	res := string(output.Payload)[1 : len(string(output.Payload))-1]

	return res
}
