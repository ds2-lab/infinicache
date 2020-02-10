package main
//
//import (
//	"encoding/json"
//	"flag"
//	"fmt"
//	"github.com/ScottMansfield/nanolog"
//	"github.com/aws/aws-sdk-go/aws"
//	"github.com/aws/aws-sdk-go/aws/session"
//	"github.com/aws/aws-sdk-go/service/lambda"
//	protocol "github.com/mason-leap-lab/infinicache/reclaim_test/types"
//	"log"
//	"os"
//	"strconv"
//	"strings"
//	"sync"
//	"sync/atomic"
//	"time"
//)
//
//type lambdaInstance struct {
//	name      string
//	timeStamp []string
//	//change     bool
//	srcChanged bool
//	repChanged bool
//}
//
//var (
//	counter = 0
//	name    = flag.String("name", "reclaim", "lambda function name")
//	num     = flag.Int("count", 1, "lambda Count")
//	m       = flag.Int64("m", 10, "periodic warmup minute")
//	h       = flag.Int64("h", 2, "total time for exp")
//	pre     = flag.String("prefix", "log", "prefix for output log file")
//	LogData = nanolog.AddLogger("%s")
//	errChan = make(chan error, 1)
//	src     int32
//	replica int32
//)
//
//func main() {
//	flag.Parse()
//	var wg sync.WaitGroup
//
//	lambdaGroup := make([]*lambdaInstance, *num)
//
//	nanoLogout, err := os.Create(fmt.Sprintf("%s_%d_%d.clog", *pre, *m, *h))
//	if err != nil {
//		panic(err)
//	}
//	err = nanolog.SetWriter(nanoLogout)
//	if err != nil {
//		panic(err)
//	}
//
//	for i := range lambdaGroup {
//		name := fmt.Sprintf("%s%s", *name, strconv.Itoa(i))
//		lambdaGroup[i] = newLambdaInstance(name)
//		log.Println("i is", name, i)
//	}
//	log.Println("initial lambda finish")
//
//	for i := range lambdaGroup {
//		wg.Add(1)
//		go lambdaTrigger(lambdaGroup[i], &wg, &src, &replica)
//	}
//	wg.Wait()
//	log.Println("==================")
//	log.Println("first trigger finished (initial)")
//	log.Println("==================")
//
//	// get timer
//	duration1 := time.Duration(*m) * time.Minute
//	//duration1 := time.Duration(*m) * time.Second
//	t := time.NewTimer(duration1)
//	duration2 := time.Duration(*h) * time.Minute
//	//duration2 := time.Duration(*h) * time.Second
//	t2 := time.NewTimer(duration2)
//
//	// start testing
//	for {
//		select {
//		case <-t.C:
//			src = 0
//			replica = 0
//			for i := range lambdaGroup {
//				wg.Add(1)
//				go lambdaTrigger(lambdaGroup[i], &wg, &src, &replica)
//			}
//			wg.Wait()
//			log.Println("=======================")
//			log.Println(counter, "interval finished,", atomic.LoadInt32(&src), "src changed,", atomic.LoadInt32(&replica), "replica changed")
//			log.Println("=======================")
//			counter = counter + 1
//			t.Reset(duration1)
//		case <-t2.C:
//			for i := range lambdaGroup {
//				res := strings.Join(lambdaGroup[i].timeStamp, ",")
//				nanolog.Log(LogData, res)
//			}
//			log.Println("warm up finished")
//			err := nanolog.Flush()
//			if err != nil {
//				fmt.Println("flush err is", err)
//			}
//			return
//		case <-errChan:
//			//nanolog.Flush()
//			log.Println("err occurred", <-errChan)
//			return
//
//		}
//	}
//
//}
//
//func newLambdaInstance(name string) *lambdaInstance {
//	return &lambdaInstance{
//		name:      name,
//		timeStamp: make([]string, 1, 100),
//		//change:    false,
//		srcChanged: false,
//		repChanged: false,
//	}
//}
//
//func lambdaTrigger(l *lambdaInstance, wg *sync.WaitGroup, src *int32, replica *int32) {
//	sess := session.Must(session.NewSessionWithOptions(session.Options{
//		SharedConfigState: session.SharedConfigEnable,
//	}))
//	client := lambda.New(sess, &aws.Config{Region: aws.String("us-east-1")})
//	event := &protocol.InputEvent{
//		Cmd: "trigger",
//	}
//	payload, _ := json.Marshal(event)
//	input := &lambda.InvokeInput{
//		FunctionName: aws.String(l.name),
//		Payload:      payload,
//	}
//	output, err := client.Invoke(input)
//	if err != nil {
//		fmt.Println("Error calling LambdaFunction", err)
//		errChan <- err
//	}
//
//	res := string(output.Payload)[1 : len(string(output.Payload))-1]
//	// this time timeStamp
//	srcTimeStamp, repTimeStamp := getTimeStamp(res)
//
//	if l.timeStamp[0] == "" {
//		l.timeStamp[0] = res
//	} else if res != l.timeStamp[len(l.timeStamp)-1] {
//		// store dat
//		l.timeStamp = append(l.timeStamp, res)
//		// get this time timestamp and previous record
//		oldSrcTimeStamp, oldRepTimeStamp := getTimeStamp(l.timeStamp[len(l.timeStamp)-1])
//		if srcTimeStamp != oldSrcTimeStamp {
//			atomic.AddInt32(src, 1)
//			l.srcChanged = true
//		}
//		if repTimeStamp != oldRepTimeStamp {
//			atomic.AddInt32(replica, 1)
//			l.repChanged = true
//		}
//	}
//	//nanolog.Log(LogData, res, *output.StatusCode)
//	log.Println(l.name, "returned", *output.StatusCode, "src changed", l.srcChanged, "replica changed", l.repChanged, srcTimeStamp, repTimeStamp)
//	l.srcChanged = false
//	l.srcChanged = false
//	wg.Done()
//}
//
//func getTimeStamp(s string) (string, string) {
//	tmp := strings.Split(s, ",")
//	return tmp[2], tmp[5]
//}
//
