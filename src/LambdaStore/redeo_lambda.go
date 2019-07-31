package main

import (
	"bytes"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/wangaoone/LambdaObjectstore/lib/logger"
	"github.com/wangaoone/redeo"
	"github.com/wangaoone/redeo/resp"
	"github.com/wangaoone/s3gof3r"
	"io"
	"net"
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

var (
	lambdaConn, _ = net.Dial("tcp", "3.220.244.122:6379") // 10Gbps ec2 server UbuntuProxy0
	//lambdaConn, _ = net.Dial("tcp", "172.31.18.174:6379") // 10Gbps ec2 server Proxy1
	srv     = redeo.NewServer(nil)
	myMap   = make(map[string]*Chunk)
	isFirst = true
	log     = logger.NilLogger
)

func HandleRequest() {
	timeOut := time.Duration(120 * time.Second)
	done := make(chan struct{})
	dataGatherer := make(chan *DataEntry, 10)
	dataDepository := make([]*DataEntry, 0, 100)

	if isFirst == true {
		isFirst = false
		go func() {
			log.Debug("conn is", lambdaConn.LocalAddr(), lambdaConn.RemoteAddr())
			// Define handlers
			srv.HandleFunc("get", func(w resp.ResponseWriter, c *resp.Command) {
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
					dataGatherer <- &DataEntry{OP_GET, "404", reqId, "-1", 0, 0, time.Since(t)}
					return
				}

				// construct lambda store response
				w.AppendBulkString(connId)
				w.AppendBulkString(reqId)
				w.AppendBulkString(chunk.id)
				t2 := time.Now()
				w.AppendBulk(chunk.body)
				d2 := time.Since(t2)

				t3 := time.Now()
				if err := w.Flush(); err != nil {
					log.Error("Error on get::flush(key %s): %v", key, err)
					dataGatherer <- &DataEntry{OP_GET, "500", reqId, chunk.id, d2, 0, time.Since(t)}
					return
				}
				d3 := time.Since(t3)

				dt := time.Since(t)

				log.Debug("AppendBody duration is ", d2)
				log.Debug("Flush duration is ", d3)
				log.Debug("Total duration is", dt)
				log.Debug("Get complete, Key: %s, ConnID:%s, ChunkID:%s", key, connId, chunk.id)
				dataGatherer <- &DataEntry{OP_GET, "200", reqId, chunk.id, d2, d3, dt}
			})

			srv.HandleFunc("set", func(w resp.ResponseWriter, c *resp.Command) {
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
				w.AppendBulkString(connId)
				w.AppendBulkString(reqId)
				w.AppendBulkString(chunkId)
				w.AppendInt(1)
				if err := w.Flush(); err != nil {
					log.Error("Error on set::flush(key %s): %v", key, err)
					dataGatherer <- &DataEntry{OP_SET, "500", reqId, chunkId, 0, 0, time.Since(t)}
					return
				}

				log.Debug("Set complete, Key:%s, ConnID: %s, ChunkID: %s, Item length", key, connId, chunkId, len(val))
				dataGatherer <- &DataEntry{OP_SET, "200", reqId, chunkId, 0, 0, time.Since(t)}
			})

			srv.HandleFunc("data", func(w resp.ResponseWriter, c *resp.Command) {
				log.Debug("in the data function")

				w.AppendBulkString("data")
				w.AppendInt(int64(len(dataDepository)))
				for _, entry := range dataDepository {
					w.AppendBulkString(entry.op)
					w.AppendBulkString(entry.status)
					w.AppendBulkString(entry.reqId)
					w.AppendBulkString(entry.chunkId)
					w.AppendInt(int64(entry.durationAppend))
					w.AppendInt(int64(entry.durationFlush))
					w.AppendInt(int64(entry.duration))
				}
				if err := w.Flush(); err != nil {
					log.Error("Error on data::flush: %v", err)
					return
				}
				log.Debug("data complete")
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
			}
		}
	}()

	// timeout control
	select {
	case <-done:
		return
	case <-time.After(timeOut):
		log.Debug("Lambda timeout, going to return function")
		return
	}
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
