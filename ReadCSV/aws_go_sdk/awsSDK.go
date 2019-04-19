package aws_go_sdk

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"log"
	"time"
)

func main() {
	//svc := s3.New(session.New())
	//input := s3.GetObjectInput{
	//	Bucket: aws.String("ao.webapp"),
	//	Key:    aws.String("200mb.csv"),
	//}
	//result, err := svc.GetObject(input)
	//if err != nil {
	//	if aerr, ok := err.(awserr.Error); ok {
	//		switch aerr.Code() {
	//		case s3.ErrCodeNoSuchKey:
	//			fmt.Println(s3.ErrCodeNoSuchKey, aerr.Error())
	//		default:
	//			fmt.Println(aerr.Error())
	//		}
	//	} else {
	//		// Print the error, cast err to awserr.Error to get the Code and
	//		// Message from an error.
	//		fmt.Println(err.Error())
	//	}
	//	return
	//}

	startDial := time.Now()
	bucket := "ao.webapp"
	item := "30mb.jpg"

	// 2) Create an AWS session
	sess, _ := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1")},
	)

	// 3) Create a new AWS S3 downloader
	downloader := s3manager.NewDownloader(sess, func(d *s3manager.Downloader) {
		d.PartSize = 20 * 1024 * 1024 // 128kb per part
		d.Concurrency = 32
	})

	fmt.Println("start downloading")

	// 4) Download the item from the bucket. If an error occurs, log it and exit. Otherwise, notify the user that the download succeeded.
	//file, _ := os.Create("Fig.jpg")
	buf := aws.NewWriteAtBuffer([]byte{})
	numBytes, err := downloader.Download(buf,
		&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(item),
		})

	if err != nil {
		log.Fatalf("Unable to download item %q, %v", item, err)
	}
	//file.Close()

	fmt.Println("Downloaded", numBytes, "bytes")
	response := time.Since(startDial)
	fmt.Println("RTT is ", response)
}
