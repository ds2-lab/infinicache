package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	lambdaService "github.com/aws/aws-sdk-go/service/lambda"
	protocol "github.com/wangaoone/LambdaObjectstore/reclaim_test/types"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"
)

type lambdaInstance struct {
	//init     bool
	name string
	//changed  bool
	//srcTime  string
	//dst1Time string
	//dst2Time string
}

var (
	counter  = 0
	name     = flag.String("name", "reclaim", "lambda function name")
	num      = flag.Int("count", 1, "lambda Count")
	m        = flag.Int64("m", 1, "periodic warmup minute")
	h        = flag.Int64("h", 2, "total time for exp")
	errChan  = make(chan error, 1)
	interval = []int{30, 60, 90, 120, 150, 180, 210, 240}
	c        = newC()
)

func newC() *lambdaService.Lambda {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	client := lambdaService.New(sess, &aws.Config{Region: aws.String("us-east-1")})
	return client

}

func main() {
	flag.Parse()
	var wg sync.WaitGroup
	//var sum int32
	lambdaGroup := make([]*lambdaInstance, *num)

	for i := range lambdaGroup {
		name := fmt.Sprintf("%s%s", *name, strconv.Itoa(i))
		lambdaGroup[i] = newLambdaInstance(name)
		//log.Println("i is", name, i)
	}
	log.Println("create lambda instance finish")

	for i := range lambdaGroup {
		wg.Add(1)
		go lambdaFullTrigger(lambdaGroup[i], &wg)
	}
	wg.Wait()
	log.Println("==================")
	log.Println("first trigger finished (initial)")
	log.Println("==================")

	// get timer
	//duration1 := gen(interval)
	duration1 := time.Duration(*m) * time.Minute
	t := time.NewTimer(duration1)
	duration2 := time.Duration(*h) * time.Minute
	t2 := time.NewTimer(duration2)
	duration3 := gen(interval)
	t3 := time.NewTimer(duration3)

	// start testing
	for {
		select {
		case <-t3.C:
			//sum = 0
			for i := range lambdaGroup {
				wg.Add(1)
				go lambdaSrcTrigger(lambdaGroup[i], &wg)
			}
			wg.Wait()
			log.Println("=======================")
			log.Println(counter, "interval finished")
			duration3 := gen(interval)
			log.Println("next interval is", duration3)
			log.Println("=======================")
			counter = counter + 1
			t3.Reset(duration3)
		case <-t.C:
			//sum = 0
			for i := range lambdaGroup {
				wg.Add(1)
				go lambdaFullTrigger(lambdaGroup[i], &wg)
			}
			wg.Wait()
			log.Println("=======================")
			log.Println(counter, "interval finished")
			log.Println("=======================")
			counter = counter + 1
			t.Reset(duration1)
		case <-t2.C:
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
		//init: true,
		name: name,
		//changed:  false,
		//srcTime:  "",
		//dst1Time: "",
		//dst2Time: "",
	}
}

func lambdaFullTrigger(l *lambdaInstance, wg *sync.WaitGroup) {
	//sess := session.Must(session.NewSessionWithOptions(session.Options{
	//	SharedConfigState: session.SharedConfigEnable,
	//}))c
	//client := lambda.New(sess, &aws.Config{Region: aws.String("us-east-1")})
	output, err := c.Invoke(&lambda.InvokeInput{FunctionName: aws.String(l.name)})
	if err != nil {
		fmt.Println("Error calling LambdaFunction, lambdaFullTrigger", l.name, err)
		errChan <- err
	}
	res := string(output.Payload)[1 : len(string(output.Payload))-1]
	log.Println(res)
	// get returned timeStamp
	ts := strings.Split(res, ",")
	_ = ts[1]
	_ = ts[3]
	_ = ts[5]

	//// func,ts,func,ts,func,ts
	//if l.init == true {
	//	l.srcTime = ts[1]
	//	l.dst1Time = ts[3]
	//	l.dst2Time = ts[5]
	//	l.init = false
	//} else {
	//	if l.srcTime == ts[1] || l.srcTime == ts[3] || l.srcTime == ts[5] ||
	//		l.dst1Time == ts[1] || l.dst1Time == ts[3] || l.dst1Time == ts[5] ||
	//		l.dst2Time == ts[1] || l.dst2Time == ts[3] || l.dst2Time == ts[5] {
	//	} else {
	//		atomic.AddInt32(s, 1)
	//		l.changed = true
	//	}
	//	// update timeStamp of src and dst
	//	l.srcTime = ts[1]
	//	l.dst1Time = ts[3]
	//	l.dst2Time = ts[5]
	//}
	//log.Println(l.name, l.changed)
	//l.changed = false
	wg.Done()
}

func lambdaSrcTrigger(l *lambdaInstance, wg *sync.WaitGroup) {
	//sess := session.Must(session.NewSessionWithOptions(session.Options{
	//	SharedConfigState: session.SharedConfigEnable,
	//}))
	//client := lambda.New(sess, &aws.Config{Region: aws.String("us-east-1")})
	input := inputEvent(l.name, "src")
	output, err := c.Invoke(input)
	if err != nil {
		fmt.Println("Error calling LambdaFunction, lambdaSrcTrigger", l.name, err)
		errChan <- err
	}
	res := string(output.Payload)[1 : len(string(output.Payload))-1]
	log.Println(res)
	// get returned timeStamp
	ts := strings.Split(res, ",")
	_ = ts[1]
	_ = ts[3]
	//
	//if l.srcTime == ts[1] || l.dst1Time == ts[1] || l.dst2Time == ts[1] ||
	//	l.srcTime == ts[3] || l.dst1Time == ts[3] || l.dst2Time == ts[3] {
	//} else {
	//	atomic.AddInt32(s, 1)
	//	l.changed = true
	//}
	// update timeStamp of src
	//l.srcTime = ts[1]
	//l.dst1Time = ts[3]
	//log.Println(l.name, l.changed)
	//l.changed = false
	wg.Done()
}

func gen(interval []int) time.Duration {
	rand.Seed(time.Now().UnixNano())
	duration := time.Duration(interval[rand.Intn(len(interval))]) * time.Second
	return duration
}

func inputEvent(name string, cmd string) *lambdaService.InvokeInput {
	event := &protocol.InputEvent{
		Cmd: cmd,
	}
	payload, _ := json.Marshal(event)
	input := &lambdaService.InvokeInput{
		FunctionName: aws.String(name),
		Payload:      payload,
	}
	return input
}
