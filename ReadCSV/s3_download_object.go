package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"os"
	"time"
)

// Downloads an item from an S3 Bucket in the region configured in the shared config
// or AWS_REGION environment variable.
//
// Usage:
//    go run s3_download_object.go BUCKET ITEM
func main() {
	start := time.Now()
	if len(os.Args) != 3 {
		exitErrorf("Bucket and item names required\nUsage: %s bucket_name item_name",
			os.Args[0])
	}

	bucket := os.Args[1]
	item := os.Args[2]

	file, err := os.Create(item)
	if err != nil {
		exitErrorf("Unable to open file %q, %v", err)
	}

	defer file.Close()

	// Initialize a session in us-west-2 that the SDK will use to load
	// credentials from the shared credentials file ~/.aws/credentials.
	sess, _ := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1")},
	)

	downloader := s3manager.NewDownloader(sess)

	numBytes, err := downloader.Download(file,
		&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(item),
		})
	if err != nil {
		exitErrorf("Unable to download item %q, %v", item, err)
	}

	fmt.Println("Downloaded", file.Name(), numBytes, "bytes")
	fmt.Println("time is ", time.Since(start))
}

func exitErrorf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}
