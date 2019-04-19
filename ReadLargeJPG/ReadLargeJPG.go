package main

import (
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"log"
	"runtime"
)

func LambdaHandler() {
	bucket := "ao.webapp"
	item := "30mb.jpg"

	// 2) Create an AWS session
	sess, _ := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1")},
	)

	// 3) Create a new AWS S3 downloader
	downloader := s3manager.NewDownloader(sess, func(d *s3manager.Downloader) {
		d.PartSize = 10 * 1024 * 1024 // 10mb per part
		d.Concurrency = 10
	})

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
	fmt.Println("CPUs", runtime.NumCPU())
}
func main() {
	lambda.Start(LambdaHandler)
}
