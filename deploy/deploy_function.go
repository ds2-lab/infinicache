package main

import (
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"math"
	"sync"
	"time"
)

const (
	ROLE = "arn:aws:iam::037862857942:role/ProxyNoVPC"
)

var (
	code    = flag.Bool("code", false, "update function code")
	config  = flag.Bool("config", false, "update function config")
	create  = flag.Bool("create", false, "create function")
	timeout = flag.Int64("timeout", 100, "function timeout")
	prefix  = flag.String("prefix", "Proxy1Node", "function name prefix")
	vpc     = flag.Bool("vpc", false, "vpc config")
	key     = flag.String("key", "lambda", "key for handler and file name")
	from    = flag.Int64("from", 0, "the number of lambda deployment involved")
	to      = flag.Int64("to", 400, "the number of lambda deployment involved")
	batch   = flag.Int64("batch", 5, "batch Number, no need to modify")
	mem     = flag.Int64("mem", 256, "the memory of lambda")
	bucket  = flag.String("S3", "ao.lambda.code", "S3 bucket for lambda code")

	subnet = []*string{
		aws.String("subnet-eeb536c0"),
		aws.String("subnet-f432faca"),
	}
	securityGroup = []*string{
		aws.String("sg-0281863209f428cb2"), aws.String("sg-d5b37d99"),
	}
)

func updateConfig(name string, svc *lambda.Lambda, wg *sync.WaitGroup) {
	var vpcConfig *lambda.VpcConfig
	if *vpc {
		vpcConfig = &lambda.VpcConfig{SubnetIds: subnet, SecurityGroupIds: securityGroup}
	} else {
		vpcConfig = &lambda.VpcConfig{}
	}
	input := &lambda.UpdateFunctionConfigurationInput{
		//Description:  aws.String(""),
		FunctionName: aws.String(name),
		Handler:      aws.String(*key),
		MemorySize:   aws.Int64(*mem),
		Role:         aws.String(ROLE),
		Timeout:      aws.Int64(*timeout),
		VpcConfig:    vpcConfig,
		//VpcConfig: &lambda.VpcConfig{SubnetIds: subnet, SecurityGroupIds: securityGroup},
		//VpcConfig: &lambda.VpcConfig{},
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
	fmt.Println(name, "\n", result)
	wg.Done()
	return
}

func updateCode(name string, svc *lambda.Lambda, wg *sync.WaitGroup) {
	input := &lambda.UpdateFunctionCodeInput{
		FunctionName: aws.String(name),
		S3Bucket:     aws.String(*bucket),
		S3Key:        aws.String(fmt.Sprintf("%s.zip", *key)),
	}
	result, err := svc.UpdateFunctionCode(input)
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
			case lambda.ErrCodeCodeStorageExceededException:
				fmt.Println(lambda.ErrCodeCodeStorageExceededException, aerr.Error())
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
	fmt.Println(name, "\n", result)
	wg.Done()
	return
}

func createFunction(name string, svc *lambda.Lambda) {
	var vpcConfig *lambda.VpcConfig
	if *vpc {
		vpcConfig = &lambda.VpcConfig{SubnetIds: subnet, SecurityGroupIds: securityGroup}
	} else {
		vpcConfig = &lambda.VpcConfig{}
	}
	input := &lambda.CreateFunctionInput{
		Code: &lambda.FunctionCode{
			S3Bucket: aws.String(*bucket),
			S3Key:    aws.String(fmt.Sprintf("%s.zip", *key)),
		},
		FunctionName: aws.String(name),
		Handler:      aws.String(*key),
		MemorySize:   aws.Int64(*mem),
		Role:         aws.String(ROLE),
		Runtime:      aws.String("go1.x"),
		Timeout:      aws.Int64(*timeout),
		VpcConfig:    vpcConfig,
	}

	result, err := svc.CreateFunction(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case lambda.ErrCodeServiceException:
				fmt.Println(lambda.ErrCodeServiceException, aerr.Error())
			case lambda.ErrCodeInvalidParameterValueException:
				fmt.Println(lambda.ErrCodeInvalidParameterValueException, aerr.Error())
			case lambda.ErrCodeResourceNotFoundException:
				fmt.Println(lambda.ErrCodeResourceNotFoundException, aerr.Error())
			case lambda.ErrCodeResourceConflictException:
				fmt.Println(lambda.ErrCodeResourceConflictException, aerr.Error())
			case lambda.ErrCodeTooManyRequestsException:
				fmt.Println(lambda.ErrCodeTooManyRequestsException, aerr.Error())
			case lambda.ErrCodeCodeStorageExceededException:
				fmt.Println(lambda.ErrCodeCodeStorageExceededException, aerr.Error())
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

	fmt.Println(result)
}

//func upload(sess *session.Session) {
//	// Create an uploader with the session and default options
//	uploader := s3manager.NewUploader(sess)
//
//	f, err := os.Open(fileName)
//	if err != nil {
//		fmt.Println("failed to open file", fileName, err)
//	}
//
//	// Upload the file to S3.
//	result, err := uploader.Upload(&s3manager.UploadInput{
//		Bucket: aws.String(BUCKET),
//		Key:    aws.String(KEY),
//		Body:   f,
//	})
//	if err != nil {
//		fmt.Println("failed to upload file", err)
//	}
//	fmt.Println("file uploaded to", result.Location)
//}

func main() {
	flag.Parse()
	// get group count
	group := int64(math.Ceil(float64(*to-*from) / float64(*batch)))
	fmt.Println("group", group)
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	svc := lambda.New(sess, &aws.Config{Region: aws.String("us-east-1")})
	if *code {
		for j := int64(0); j < group; j++ {
			fmt.Println(j)
			var wg sync.WaitGroup
			//for i := j*(*batch) + *from; i < (j+1)*(*batch); i++ {
			for i := int64(0); i < *batch; i++ {
				wg.Add(1)
				go updateCode(fmt.Sprintf("%s%d", *prefix, j*(*batch)+*from+i), svc, &wg)
			}
			wg.Wait()
			time.Sleep(1 * time.Second)
		}
	}
	if *config {
		for j := int64(0); j < group; j++ {
			fmt.Println(j)
			var wg sync.WaitGroup
			for i := int64(0); i < *batch; i++ {
				wg.Add(1)
				go updateConfig(fmt.Sprintf("%s%d", *prefix, j*(*batch)+*from+i), svc, &wg)
			}
			wg.Wait()
			time.Sleep(1 * time.Second)
		}
	}
	if *create {
		for i := *from; i < *to; i++ {
			createFunction(fmt.Sprintf("%s%d", *prefix, i), svc)
		}
	}
}
