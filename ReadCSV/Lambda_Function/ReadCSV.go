package Lambda_Function

import (
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"log"
)

//func LambdaHandler1() {
//	svc := s3.New(session.New())
//	input := &s3.GetObjectInput{
//		Bucket: aws.String("ao.webapp"),
//		Key:    aws.String("200mb.csv"),
//	}
//	result, err := svc.GetObject(input)
//	if err != nil {
//		if aerr, ok := err.(awserr.Error); ok {
//			switch aerr.Code() {
//			case s3.ErrCodeNoSuchKey:
//				fmt.Println(s3.ErrCodeNoSuchKey, aerr.Error())
//			default:
//				fmt.Println(aerr.Error())
//			}
//		} else {
//			// Print the error, cast err to awserr.Error to get the Code and
//			// Message from an error.
//			fmt.Println(err.Error())
//		}
//		return
//	}
//	fmt.Println("load successful")
//	fmt.Println(result)
//}

//func LambdaHandler2() {
//	svc := s3.New(session.New())
//	input := s3.GetObjectInput{
//		Bucket: aws.String("ao.webapp"),
//		Key:    aws.String("200mb.csv"),
//	}
//	//result, err := svc.GetObject(input)
//	//if err != nil {
//	//	if aerr, ok := err.(awserr.Error); ok {
//	//		switch aerr.Code() {
//	//		case s3.ErrCodeNoSuchKey:
//	//			fmt.Println(s3.ErrCodeNoSuchKey, aerr.Error())
//	//		default:
//	//			fmt.Println(aerr.Error())
//	//		}
//	//	} else {
//	//		// Print the error, cast err to awserr.Error to get the Code and
//	//		// Message from an error.
//	//		fmt.Println(err.Error())
//	//	}
//	//	return
//	//}
//	fmt.Println("load successful")
//	fmt.Println(input)
//
//	buf := aws.NewWriteAtBuffer([]byte{})
//	downloader := s3manager.NewDownloaderWithClient(svc)
//
//	downloader.Download(buf, &input)
//	fmt.Printf("Downloaded %v bytes", len(buf.Bytes()))
//}

func LambdaHandler() {
	bucket := "ao.webapp"
	item := "200mb.csv"

	// 2) Create an AWS session
	sess, _ := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1")},
	)

	// 3) Create a new AWS S3 downloader
	downloader := s3manager.NewDownloader(sess)

	// 4) Download the item from the bucket. If an error occurs, log it and exit. Otherwise, notify the user that the download succeeded.
	buf := aws.NewWriteAtBuffer([]byte{})
	numBytes, err := downloader.Download(buf,
		&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(item),
		})

	if err != nil {
		log.Fatalf("Unable to download item %q, %v", item, err)
	}

	fmt.Println("Downloaded", numBytes, "bytes")
}
func main() {
	lambda.Start(LambdaHandler)
}
