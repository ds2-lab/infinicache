package main

import (
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"os/exec"
	"strings"
	"time"
)

var (
	t        = time.Now().UnixNano()
	hostName string
)

func init() {
	cmd := exec.Command("uname", "-a")
	host, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("cmd.Run() failed with ", err)
	}
	hostName = strings.Split(string(host), " #")[0]
}
func HandleRequest() (string, error) {

	fmt.Println("deployment is", lambdacontext.FunctionName, "hostName is", hostName, "Global timeStamp is", t)
	return fmt.Sprintf("%s,%s,%d", lambdacontext.FunctionName, hostName, t), nil
}

func main() {
	lambda.Start(HandleRequest)
}
