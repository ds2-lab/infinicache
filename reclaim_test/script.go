package main

import (
	"flag"
	"fmt"
	"github.com/ScottMansfield/nanolog"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"log"
	"os"
	"strconv"
	"sync"
	"time"
)

type lambdaInstance struct {
	name string
}

var (
	name    = flag.String("name", "reclaim", "lambda function name")
	num     = flag.Int("count", 1, "lambda Count")
	m       = flag.Int64("m", 10, "periodic warmup minute")
	h       = flag.Int64("h", 2, "total time for exp")
	pre     = flag.String("prefix", "log", "prefix for output log file")
	LogData = nanolog.AddLogger("%s,%i64")
	errChan = make(chan error, 1)
)

func main() {
	flag.Parse()
	var wg sync.WaitGroup
	lambdaGroup := make([]*lambdaInstance, *num)

	nanoLogout, err := os.Create(*pre + ".clog")
	if err != nil {
		panic(err)
	}
	err = nanolog.SetWriter(nanoLogout)
	if err != nil {
		panic(err)
	}

	for i := range lambdaGroup {
		name := fmt.Sprintf("%s%s", *name, strconv.Itoa(i))
		lambdaGroup[i] = newLambdaInstance(name)
		log.Println("i is", name, i)
	}
	log.Println("initial lambda finish")

	for i := range lambdaGroup {
		wg.Add(1)
		go lambdaTrigger(lambdaGroup[i], &wg)
	}
	wg.Wait()
	log.Println("==================")
	log.Println("interval finished")
	log.Println("==================")

	t := time.NewTicker(time.Duration(*m) * time.Minute)
	//t := time.NewTicker(30 * time.Second)
	t2 := time.NewTicker(time.Duration(*h) * time.Minute)
	//t2 := time.NewTicker(2 * time.Minute)
	for {
		select {
		case <-t.C:
			for i := range lambdaGroup {
				wg.Add(1)
				go lambdaTrigger(lambdaGroup[i], &wg)
			}
			wg.Wait()
			log.Println("==================")
			log.Println("interval finished")
			log.Println("==================")
		case <-t2.C:
			nanolog.Flush()
			log.Println("warm up finished")
			return
		case <-errChan:
			//nanolog.Flush()
			log.Println("err occurred", <-errChan)
			return

		}
	}

}

func newLambdaInstance(name string) *lambdaInstance {
	return &lambdaInstance{
		name: name,
	}
}

func lambdaTrigger(l *lambdaInstance, wg *sync.WaitGroup) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	client := lambda.New(sess, &aws.Config{Region: aws.String("us-east-1")})
	output, err := client.Invoke(&lambda.InvokeInput{FunctionName: aws.String(l.name)})
	if err != nil {
		fmt.Println("Error calling LambdaFunction", err)
		errChan <- err
	}
	log.Println(l.name, "returned, status code is", *output.StatusCode)

	res := string(output.Payload)
	res = res[1 : len(res)-1]

	nanolog.Log(LogData, res, *output.StatusCode)
	wg.Done()
}
