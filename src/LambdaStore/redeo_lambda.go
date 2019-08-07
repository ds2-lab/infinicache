package main

import (
	"bytes"
	"fmt"
	"math"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/wangaoone/LambdaObjectstore/lib/logger"
	"github.com/wangaoone/redeo"
	"github.com/wangaoone/redeo/resp"
	"github.com/wangaoone/s3gof3r"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type Chunk struct {
	id   string
	body []byte
}

type DataEntry struct {
	op             string
	status         string
	reqId          string
	chunkId        string
	durationAppend time.Duration
	durationFlush  time.Duration
	duration       time.Duration
}

const OP_GET = "1"
const OP_SET = "0"
const TICK = int64(100 * time.Millisecond)
const TICK_ERROR = int64(2 * time.Millisecond)

var (
	//server     = "184.73.144.223:6379" // 10Gbps ec2 server UbuntuProxy0
	server = "172.31.84.57:6379" // t2.micro ec2 server UbuntuProxy0 private ip under vpc
	lambdaConn net.Conn
	srv     = redeo.NewServer(nil)
	myMap   = make(map[string]*Chunk)
	isFirst = true
	log     = &logger.ColorLogger{
		Level: logger.LOG_LEVEL_WARN,
	}
	start time.Time
)

func HandleRequest() {
	var active int32
	start = time.Now()
	timeOut := time.NewTimer(getTimeout())
	done := make(chan struct{})
	dataGatherer := make(chan *DataEntry, 10)
	dataDepository := make([]*DataEntry, 0, 100)
	var dataDeposited sync.WaitGroup

	if isFirst == true {
		log.Debug("Ready to connect %s", server)

		var connErr error
		lambdaConn, connErr = net.Dial("tcp", server)
		if connErr != nil {
			log.Error("Failed to connect server %s: %v", server, connErr)
			return
		}
		log.Info("Connection to %v established.", lambdaConn.RemoteAddr())

		isFirst = false
		go func() {
			// Define handlers
			srv.HandleFunc("get", func(w resp.ResponseWriter, c *resp.Command) {
				atomic.AddInt32(&active, 1)
				t := time.Now()
				log.Debug("In GET handler")

				connId := c.Arg(0).String()
				reqId := c.Arg(1).String()
				key := c.Arg(3).String()

				//val, err := myCache.Get(key)
				//if err == false {
				//	log.Debug("not found")
				//}
				chunk, found := myMap[key]
				if found == false {
					log.Debug("%s not found", key)
					dataDeposited.Add(1)
					dataGatherer <- &DataEntry{OP_GET, "404", reqId, "-1", 0, 0, time.Since(t)}
					return
				}

				// construct lambda store response
				w.AppendBulkString("get")
				w.AppendBulkString(connId)
				w.AppendBulkString(reqId)
				w.AppendBulkString(chunk.id)
				t2 := time.Now()
				w.AppendBulk(chunk.body)
				d2 := time.Since(t2)

				t3 := time.Now()
				if err := w.Flush(); err != nil {
					log.Error("Error on get::flush(key %s): %v", key, err)
					dataDeposited.Add(1)
					dataGatherer <- &DataEntry{OP_GET, "500", reqId, chunk.id, d2, 0, time.Since(t)}
					atomic.AddInt32(&active, -1)
					resetTimer(timeOut)
					return
				}
				d3 := time.Since(t3)
				dt := time.Since(t)

				log.Debug("AppendBody duration is ", d2)
				log.Debug("Flush duration is ", d3)
				log.Debug("Total duration is", dt)
				log.Debug("Get complete, Key: %s, ConnID:%s, ChunkID:%s", key, connId, chunk.id)
				dataDeposited.Add(1)
				dataGatherer <- &DataEntry{OP_GET, "200", reqId, chunk.id, d2, d3, dt}
				atomic.AddInt32(&active, -1)
				resetTimer(timeOut)
			})

			srv.HandleFunc("set", func(w resp.ResponseWriter, c *resp.Command) {
				atomic.AddInt32(&active, 1)
				t := time.Now()
				log.Debug("In SET handler")
				//if c.ArgN() != 3 {
				//	w.AppendError(redeo.WrongNumberOfArgs(c.Name))
				//	return
				//}

				connId := c.Arg(0).String()
				reqId := c.Arg(1).String()
				chunkId := c.Arg(2).String()
				key := c.Arg(3).String()
				val := c.Arg(4).Bytes()
				myMap[key] = &Chunk{chunkId, val}

				// write Key, clientId, chunkId, body back to server
				w.AppendBulkString("set")
				w.AppendBulkString(connId)
				w.AppendBulkString(reqId)
				w.AppendBulkString(chunkId)
				if err := w.Flush(); err != nil {
					log.Error("Error on set::flush(key %s): %v", key, err)
					dataDeposited.Add(1)
					dataGatherer <- &DataEntry{OP_SET, "500", reqId, chunkId, 0, 0, time.Since(t)}
					atomic.AddInt32(&active, -1)
					resetTimer(timeOut)
					return
				}

				log.Debug("Set complete, Key:%s, ConnID: %s, ChunkID: %s, Item length %d", key, connId, chunkId, len(val))
				dataDeposited.Add(1)
				dataGatherer <- &DataEntry{OP_SET, "200", reqId, chunkId, 0, 0, time.Since(t)}
				atomic.AddInt32(&active, -1)
				resetTimer(timeOut)
			})

			srv.HandleFunc("data", func(w resp.ResponseWriter, c *resp.Command) {
				log.Debug("in the data function")

				timeOut.Stop()

				dataDeposited.Wait()

				w.AppendBulkString("data")
				w.AppendBulkString(strconv.Itoa(len(dataDepository)))
				for _, entry := range dataDepository {
					format := fmt.Sprintf("%s,%s,%s,%s,%d,%d,%d",
						entry.op, entry.reqId, entry.chunkId, entry.status,
						entry.duration, entry.durationAppend, entry.durationFlush)
					w.AppendBulkString(format)

					//w.AppendBulkString(entry.op)
					//w.AppendBulkString(entry.status)
					//w.AppendBulkString(entry.reqId)
					//w.AppendBulkString(entry.chunkId)
					//w.AppendBulkString(entry.durationAppend.String())
					//w.AppendBulkString(entry.durationFlush.String())
					//w.AppendBulkString(entry.duration.String())
				}
				if err := w.Flush(); err != nil {
					log.Error("Error on data::flush: %v", err)
					return
				}
				log.Debug("data complete")
				lambdaConn.Close()
				lambdaConn = nil
				// No need to close server, it will serve the new connection next time.
				isFirst = true
				close(done)
			})

			srv.Serve_client(lambdaConn)
		}()
	}

	// data gathering
	go func() {
		for {
			select {
			case <-done:
				return
			case entry := <-dataGatherer:
				dataDepository = append(dataDepository, entry)
				dataDeposited.Done()
			}
		}
	}()

	// timeout control
	for {
		select {
		case <-done:
			return
		case <-timeOut.C:
			if atomic.LoadInt32(&active) > 0 {
				resetTimer(timeOut)
				break
			}
			log.Debug("Lambda timeout, going to return function")
			return
		}
	}
}

func getTimeout() time.Duration {
	now := time.Now().Sub(start).Nanoseconds()
	return time.Duration(int64(math.Ceil(float64(now + TICK_ERROR) / float64(TICK))) * TICK - TICK_ERROR - now)
}

func resetTimer(timer *time.Timer) {
	// Drain the timer to be accurate and safe to reset.
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timeout := getTimeout()
	timer.Reset(timeout)
	log.Debug("Timeout reset: %v", timeout)
}

func remoteGet(bucket string, key string) []byte {
	log.Debug("get from remote storage")
	k, err := s3gof3r.EnvKeys()
	if err != nil {
		log.Debug("%v", err)
	}

	s3 := s3gof3r.New("", k)
	b := s3.Bucket(bucket)

	reader, _, err := b.GetReader(key, nil)
	if err != nil {
		log.Debug("%v", err)
	}
	obj := streamToByte(reader)
	return obj
}

func streamToByte(stream io.Reader) []byte {
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(stream)
	if err != nil {
		log.Debug("%v", err)
	}
	return buf.Bytes()
}

func main() {
	lambda.Start(HandleRequest)
}
