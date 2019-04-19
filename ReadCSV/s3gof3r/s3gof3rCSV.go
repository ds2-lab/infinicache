package main

import (
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/rlmcpherson/s3gof3r"
	"io"
	"os"
	"runtime"
	"time"
)

func LambdaHandler() {
	start := time.Now()
	file, err := os.Create("/tmp/local.csv")
	if err != nil {
		fmt.Println("Unable to open file %q, %v", err)
	}

	defer file.Close()

	s3gof3r.DefaultConfig.Md5Check = false
	s3gof3r.DefaultConfig.Concurrency = 32

	k, err := s3gof3r.EnvKeys() // get S3 keys from environment
	if err != nil {
		fmt.Println(err)
	}

	// Open bucket to put file into
	s3 := s3gof3r.New("", k)
	b := s3.Bucket("ao.webapp")

	r, h, err := b.GetReader("200mb.csv", nil)
	if err != nil {
		fmt.Println(err)
	}
	// stream to standard output
	if _, err = io.Copy(file, r); err != nil {
		fmt.Println(err)
	}
	err = r.Close()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(h) // print key header data
	fmt.Println(time.Since(start))
	fmt.Println("CPUs", runtime.NumCPU())
}
func main() {
	lambda.Start(LambdaHandler)
}
