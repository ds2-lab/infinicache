package main

import (
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"log"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type lambdaInstance struct {
	init     bool
	name     string
	changed  bool
	srcTime  string
	dst1Time string
	dst2Time string
}

var (
	counter = 0
	name    = flag.String("name", "reclaim", "lambda function name")
	num     = flag.Int("count", 1, "lambda Count")
	m       = flag.Int64("m", 1, "periodic warmup minute")
	h       = flag.Int64("h", 2, "total time for exp")
	errChan = make(chan error, 1)
)

func main() {
	flag.Parse()
	var wg sync.WaitGroup
	var sum int32
	lambdaGroup := make([]*lambdaInstance, *num)

	for i := range lambdaGroup {
		name := fmt.Sprintf("%s%s", *name, strconv.Itoa(i))
		lambdaGroup[i] = newLambdaInstance(name)
		log.Println("i is", name, i)
	}
	log.Println("create lambda instance finish")

	for i := range lambdaGroup {
		wg.Add(1)
		go lambdaTrigger(lambdaGroup[i], &wg, &sum)
	}
	wg.Wait()
	log.Println("==================")
	log.Println("first trigger finished (initial)")
	log.Println("==================")

	// get timer
	duration1 := time.Duration(*m) * time.Minute
	t := time.NewTimer(duration1)
	duration2 := time.Duration(*h) * time.Minute
	t2 := time.NewTimer(duration2)

	// start testing
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
			log.Println(counter, "interval finished", atomic.LoadInt32(&sum), "changed")
			log.Println("=======================")
			counter = counter + 1
			t.Reset(duration1)
		case <-t2.C:
			//for i := range lambdaGroup {
			//	res := strings.Join(lambdaGroup[i].timeStamp, ",")
			//	nanolog.Log(LogData, res)
			//}
			log.Println("warm up finished")
			//err := nanolog.Flush()
			//if err != nil {
			//	fmt.Println("flush err is", err)
			//}
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
		init:    true,
		name:    name,
		changed: false,
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
	res := string(output.Payload)[1 : len(string(output.Payload))-1]

	// get returned timeStamp
	ts := strings.Split(res, ",")

	if l.init == true {
		l.srcTime = ts[1]
		l.dst1Time = ts[3]
		l.dst2Time = ts[5]
		l.init = false
	} else {
		//if l.srcTime != ts[1] && l.dst1Time != ts[3] && l.dst2Time != ts[5] {
		//	if l.srcTime != ts[3] && l.dst1Time != ts[1] && l.dst2Time != ts[5] {
		//		if l.srcTime != ts[3] && l.dst1Time != ts[1] && l.dst2Time != ts[5] {
		//			atomic.AddInt32(s, 1)
		//			l.changed = true
		//		}
		//	}
		if l.srcTime == ts[1] || l.srcTime == ts[3] || l.srcTime == ts[5] ||
			l.dst1Time == ts[1] || l.dst1Time == ts[3] || l.dst1Time == ts[5] ||
			l.dst2Time == ts[1] || l.dst2Time == ts[3] || l.dst2Time == ts[5] {
		} else {
			atomic.AddInt32(s, 1)
			l.changed = true
		}
		// update timeStamp of src and dst
		l.srcTime = ts[1]
		l.dst1Time = ts[3]
		l.dst2Time = ts[5]
	}
	log.Println(res, l.changed)
	l.changed = false
	wg.Done()
}
