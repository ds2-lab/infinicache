package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"strconv"
)

var (
	prefix  = "reclaim"
	count   = 1000
	mem     = int64(3008)
	timeout = int64(120)
	subnet  = []*string{
		aws.String("subnet-eeb536c0"),
		aws.String("subnet-f94739f6"),
		aws.String("subnet-f432faca")}
	securityGroup = []*string{
		aws.String("sg-0281863209f428cb2"), aws.String("sg-d5b37d99")}
)

func update(name string, svc *lambda.Lambda) {
	input := &lambda.UpdateFunctionConfigurationInput{
		//Description:  aws.String(""),
		FunctionName: aws.String(name),
		Handler:      aws.String("reclaim"),
		MemorySize:   aws.Int64(mem),
		//Role:         aws.String("arn:aws:iam::123456789012:role/lambda_basic_execution"),
		//Runtime:      aws.String("python2.7"),
		Timeout:   aws.Int64(timeout),
		VpcConfig: &lambda.VpcConfig{SubnetIds: subnet, SecurityGroupIds: securityGroup},
	}
	result, err := svc.UpdateFunctionConfiguration(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case lambda.ErrCodeServiceException:
				fmt.Println(lambda.ErrCodeServiceException, aerr.Error())
			case lambda.ErrCodeResourceNotFoundException:
				fmt.Println(lambda.ErrCodeResourceNotFoundException, aerr.Error())
			case lambda.ErrCodeInvalidParameterValueException:
				fmt.Println(lambda.ErrCodeInvalidParameterValueException, aerr.Error())
			case lambda.ErrCodeTooManyRequestsException:
				fmt.Println(lambda.ErrCodeTooManyRequestsException, aerr.Error())
			case lambda.ErrCodeResourceConflictException:
				fmt.Println(lambda.ErrCodeResourceConflictException, aerr.Error())
			case lambda.ErrCodePreconditionFailedException:
				fmt.Println(lambda.ErrCodePreconditionFailedException, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		return
	}
	fmt.Println(name, result)
	//wg.Done()
}

func main() {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	svc := lambda.New(sess, &aws.Config{Region: aws.String("us-east-1")})

	//var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		//wg.Add(1)
		update(prefix+strconv.Itoa(i), svc)
	}
	//wg.Wait()
	fmt.Println("finish")

}
