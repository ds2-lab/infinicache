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
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type lambdaInstance struct {
	name string
	//touch        string
	//firstChange  string
	//secondChange string
	timeStamp []string
	change    bool
}

var (
	counter = 0
	name    = flag.String("name", "reclaim", "lambda function name")
	num     = flag.Int("count", 1, "lambda Count")
	m       = flag.Int64("m", 10, "periodic warmup minute")
	h       = flag.Int64("h", 2, "total time for exp")
	pre     = flag.String("prefix", "log", "prefix for output log file")
	LogData = nanolog.AddLogger("%s")
	errChan = make(chan error, 1)
)

func main() {
	flag.Parse()
	var wg sync.WaitGroup
	var sum int32
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
		go lambdaTrigger(lambdaGroup[i], &wg, &sum)
	}
	wg.Wait()
	log.Println("==================")
	log.Println("first trigger finished (initial)")
	log.Println("==================")

	t := time.NewTicker(time.Duration(*m) * time.Minute)
	//t := time.NewTicker(30 * time.Second)
	t2 := time.NewTicker(time.Duration(*h) * time.Minute)
	//t2 := time.NewTicker(2 * time.Minute)
	for {
		select {
		case <-t.C:
			sum = 0
			for i := range lambdaGroup {
				wg.Add(1)
				go lambdaTrigger(lambdaGroup[i], &wg, &sum)
			}
			wg.Wait()
			log.Println("=======================")
			log.Println(counter, "interval finished", atomic.LoadInt32(&sum), "changed timeStamp")
			log.Println("=======================")
			counter = counter + 1
		case <-t2.C:
			for i := range lambdaGroup {
				res := strings.Join(lambdaGroup[i].timeStamp, ",")
				nanolog.Log(LogData, res)
			}
			log.Println("warm up finished")
			err := nanolog.Flush()
			if err != nil {
				fmt.Println("flush err is", err)
			}
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
		name:      name,
		timeStamp: make([]string, 1, 500),
		change:    false,
	}
}

func lambdaTrigger(l *lambdaInstance, wg *sync.WaitGroup, s *int32) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	client := lambda.New(sess, &aws.Config{Region: aws.String("us-east-1")})
	output, err := client.Invoke(&lambda.InvokeInput{FunctionName: aws.String(l.name)})
	if err != nil {
		fmt.Println("Error calling LambdaFunction", err)
		errChan <- err
	}

	res := string(output.Payload)
	res = res[1 : len(res)-1]

	//if l.touch == "" {
	//	l.touch = res
	//} else {
	//	if l.touch != res && l.firstChange == "" {
	//		l.firstChange = res
	//	}
	//	if l.touch != res && l.firstChange != res && l.secondChange == "" {
	//		l.secondChange = res
	//	}
	//}
	if l.timeStamp[0] == "" {
		l.timeStamp[0] = res
	} else if res != l.timeStamp[len(l.timeStamp)-1] {
		l.change = true
		atomic.AddInt32(s, 1)
		l.timeStamp = append(l.timeStamp, res)
	}

	//nanolog.Log(LogData, res, *output.StatusCode)
	log.Println(l.name, "returned, status code is", *output.StatusCode, "timeStamp changed", l.change)
	l.change = false
	wg.Done()
}
