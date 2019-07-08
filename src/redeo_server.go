package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/cornelk/hashmap"
	"github.com/klauspost/reedsolomon"
	"github.com/wangaoone/redeo"
	"github.com/wangaoone/redeo/resp"
	"net"
	"strconv"
	"strings"
	"time"
)

var (
	multiDeployment = flag.Bool("multiDeployment", false, "enable multi-lambda deployment")
	isPrint         = flag.Bool("isPrint", true, "enable print log")
)

var (
	clientLis    net.Listener
	lambdaLis    net.Listener
	cMap         = make(map[int]chan interface{}) // client channel mapping table
	mappingTable = hashmap.New(1024)              // lambda store mapping table
)

func main() {
	flag.Parse()
	fmt.Println("======================================")
	fmt.Println("multiDeployment:", *multiDeployment, "||", "isPrint:", *isPrint)
	fmt.Println("======================================")
	clientLis, _ = net.Listen("tcp", ":6378")
	lambdaLis, _ = net.Listen("tcp", ":6379")
	fmt.Println("start listening client face port 6378")
	fmt.Println("start listening lambda face port 6379")
	srv := redeo.NewServer(nil)
	lambdaSrv := redeo.NewServer(nil)

	// initial lambda store group
	initial(lambdaSrv)

	// Start serving (blocking)
	err := srv.MyServe(clientLis, cMap, mappingTable)
	if err != nil {
		fmt.Println(err)
	}
}

func decoding(data [][]byte) string {
	enc, err := reedsolomon.New(redeo.DataShards, redeo.ParityShards, reedsolomon.WithMaxGoroutines(redeo.ECMaxGoroutine))
	if err != nil {
		fmt.Println(err)
	}
	//t1 := time.Now()
	//ok, err := enc.Verify(data)
	//fmt.Println("verify time 1 is", time.Since(t1))
	//if ok {
	//	fmt.Println("No reconstruction needed")
	//} else {
	//	fmt.Println("Verification failed. Reconstructing data")
	//	t2 := time.Now()
	//	err = enc.Reconstruct(data)
	//	fmt.Println("reconstruct time is", time.Since(t2))
	//	if err != nil {
	//		fmt.Println("Reconstruct failed -", err)
	//	}
	//	t3 := time.Now()
	//	ok, err = enc.Verify(data)
	//	fmt.Println("verify time 2 is", time.Since(t3))
	//	if !ok {
	//		fmt.Println("Verification failed after reconstruction, data likely corrupted.")
	//	}
	//	if err != nil {
	//		fmt.Println(err)
	//	}
	//}
	t2 := time.Now()
	err = enc.Reconstruct(data)
	fmt.Println("reconstruct time is", time.Since(t2))
	if err != nil {
		fmt.Println("Reconstruct failed -", err)
	}
	var res bytes.Buffer
	t4 := time.Now()
	err = enc.Join(&res, data, len(data[0])*redeo.DataShards)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("join time is", time.Since(t4))
	return res.String()
}

// initial lambda group
func initial(lambdaSrv *redeo.Server) {
	if *multiDeployment == false {
		group := redeo.Group{Arr: make([]redeo.LambdaInstance, redeo.DataShards+redeo.ParityShards), ChunkTable: make(map[redeo.Index][][]byte),
			C: make(chan redeo.Response, 1024*1024), MemCounter: 0, ChunkCounter: make(map[redeo.Index]int)}
		for i := range group.Arr {
			node := newLambdaInstance("Lambda2SmallJPG")
			myPrint("No.", i, "replication lambda store has registered")
			// register lambda instance to group
			group.Arr[i] = *node
			node.Alive = true
			go lambdaTrigger(node)
			// start a new server to receive conn from lambda store
			myPrint("start a new conn")
			node.Cn = lambdaSrv.Accept(lambdaLis)
			myPrint("lambda store has connected", node.Cn.RemoteAddr())
			// wrap writer and reader
			node.W = resp.NewRequestWriter(node.Cn)
			node.R = resp.NewResponseReader(node.Cn)
			// lambda handler
			go lambdaHandler(node)
			// lambda facing peeking response type
			go LambdaPeek(node)
			myPrint(node.Alive)
		}
		mappingTable.Set(0, &group)
	} else {
		group := redeo.Group{Arr: make([]redeo.LambdaInstance, redeo.DataShards+redeo.ParityShards), ChunkTable: make(map[redeo.Index][][]byte),
			C: make(chan redeo.Response, 1024*1024), MemCounter: 0, ChunkCounter: make(map[redeo.Index]int)}
		for i := range group.Arr {
			node := newLambdaInstance("Node" + strconv.Itoa(i))
			myPrint(node.Name, "lambda store has registered")
			// register lambda instance to group
			group.Arr[i] = *node
			node.Alive = true
			go lambdaTrigger(node)
			// start a new server to receive conn from lambda store
			myPrint("start a new conn")
			node.Cn = lambdaSrv.Accept(lambdaLis)
			myPrint("lambda store has connected", node.Cn.RemoteAddr())
			// wrap writer and reader
			node.W = resp.NewRequestWriter(node.Cn)
			node.R = resp.NewResponseReader(node.Cn)
			// lambda handler
			go lambdaHandler(node)
			// lambda facing peeking response type
			go LambdaPeek(node)
			myPrint(node.Alive)
		}
		mappingTable.Set(0, &group)
	}

}

// create new lambda instance
func newLambdaInstance(name string) *redeo.LambdaInstance {
	return &redeo.LambdaInstance{
		//name:  "dataNode" + strconv.Itoa(id),
		Name:  name,
		Alive: false,
		C:     make(chan redeo.Req, 1024*1024),
		Peek:  make(chan redeo.Response, 1024*1024),
	}
}

// blocking on lambda peek Type
// lambda handle incoming lambda store response
//
// field 0 : obj key
// field 1 : client id
// field 2 : req id
// field 3 : chunk id
// field 4 : obj val

func LambdaPeek(l *redeo.LambdaInstance) {
	for {
		var obj redeo.Response
		//
		// field 0 for obj key
		// bulkString
		field0, err := l.R.PeekType()
		if err != nil {
			fmt.Println("field0 err", err)
			return
		}
		switch field0 {
		case resp.TypeBulk:
			obj.Key, _ = l.R.ReadBulkString()
		case resp.TypeError:
			err, _ := l.R.ReadError()
			fmt.Println("peek type err0 is", err)
		default:
			panic("unexpected response type")
		}
		// get mapping table with key
		group, ok := mappingTable.Get(0)
		if ok == false {
			fmt.Println("get lambda instance failed")
		}
		//
		// field 1 for client id
		// bulkString
		field1, err := l.R.PeekType()
		if err != nil {
			fmt.Println("field1 err", err)
			return
		}
		switch field1 {
		case resp.TypeInt:
			clientId, _ := l.R.ReadInt()
			obj.Id.ClientId = int(clientId)
		case resp.TypeError:
			err, _ := l.R.ReadError()
			fmt.Println("peek type err1 is", err)
		default:
			panic("unexpected response type")
		}
		//
		// field 2 for req id
		// bulkString
		field2, err := l.R.PeekType()
		if err != nil {
			fmt.Println("field2 err", err)
			return
		}
		switch field2 {
		case resp.TypeInt:
			reqId, _ := l.R.ReadInt()
			obj.Id.ReqId = int(reqId)
		case resp.TypeError:
			err, _ := l.R.ReadError()
			fmt.Println("peek type err2 is", err)
		default:
			panic("unexpected response type")
		}
		//
		// field 3 for chunk id
		// Int
		field3, err := l.R.PeekType()
		if err != nil {
			fmt.Println("field3 err", err)
			return
		}
		switch field3 {
		case resp.TypeInt:
			chunkId, _ := l.R.ReadInt()
			obj.Id.ChunkId = int(chunkId)
		case resp.TypeError:
			err, _ := l.R.ReadError()
			fmt.Println("peek type err3 is", err)
		default:
			panic("unexpected response type")
		}
		//
		// field 4 for obj body
		// bulkString
		field4, err := l.R.PeekType()
		if err != nil {
			fmt.Println("field4 err", err)
			return
		}
		switch field4 {
		case resp.TypeBulk:
			// initialize index
			index := redeo.Index{ClientId: obj.Id.ClientId, ReqId: obj.Id.ReqId}
			// whether the [][]byte is existed with this index
			// if not, create it
			group.(*redeo.Group).Lock.Lock()
			_, found := group.(*redeo.Group).ChunkTable[index]
			if found == false {
				group.(*redeo.Group).ChunkTable[index] = make([][]byte, redeo.DataShards+redeo.ParityShards)
				group.(*redeo.Group).ChunkCounter[index] = 0
				fmt.Println("not found existed obj Id", "<", obj.Id.ClientId, obj.Id.ReqId, ">")
			}
			group.(*redeo.Group).Lock.Unlock()
			// read chunk from lambda store
			group.(*redeo.Group).ChunkTable[index][obj.Id.ChunkId], _ = l.R.ReadBulk(nil)
			// whether the [][]byte with this index is full
			group.(*redeo.Group).Lock.Lock()
			fmt.Println("client, reqId,chunk ", obj.Id.ClientId, obj.Id.ReqId, obj.Id.ChunkId)
			group.(*redeo.Group).ChunkCounter[index] += 1
			if group.(*redeo.Group).ChunkCounter[index] == 10 {
				t := time.Now()
				res := decoding(group.(*redeo.Group).ChunkTable[index])
				fmt.Println("decoding time is", time.Since(t))
				cMap[obj.Id.ClientId] <- res
				//delete(group.(*redeo.Group).ChunkTable, index)
			}
			group.(*redeo.Group).Lock.Unlock()
		case resp.TypeInt:
			index := redeo.Index{ClientId: obj.Id.ClientId, ReqId: obj.Id.ReqId}
			group.(*redeo.Group).Lock.Lock()
			_, found := group.(*redeo.Group).ChunkTable[index]
			if found == false {
				group.(*redeo.Group).ChunkTable[index] = make([][]byte, redeo.DataShards+redeo.ParityShards)
				group.(*redeo.Group).ChunkCounter[index] = 0
				fmt.Println("not found existed obj Id", "<", obj.Id.ClientId, obj.Id.ReqId, ">")
			}
			group.(*redeo.Group).Lock.Unlock()
			_, err = l.R.ReadInt()
			if err != nil {
				fmt.Println("read int err")
			}
			group.(*redeo.Group).ChunkTable[index][obj.Id.ChunkId] = []byte{1}
			group.(*redeo.Group).Lock.Lock()
			//if isFull(group.(*redeo.Group).ChunkTable[index]) {
			fmt.Println("client, reqId,chunk ", obj.Id.ClientId, obj.Id.ReqId, obj.Id.ChunkId)
			group.(*redeo.Group).ChunkCounter[index] += 1
			if group.(*redeo.Group).ChunkCounter[index] == redeo.DataShards+redeo.ParityShards {
				//delete(group.(*redeo.Group).ChunkTable, index)
				cMap[obj.Id.ClientId] <- string(1)
			}
			group.(*redeo.Group).Lock.Unlock()
		case resp.TypeError:
			err, _ := l.R.ReadError()
			fmt.Println("peek type err4 is", err)
			return
		default:
			panic("unexpected response type")
		}
	}
}

// lambda Handler
// lambda handle incoming client request
func lambdaHandler(l *redeo.LambdaInstance) {
	fmt.Println("conn is", l.Cn)
	for {
		select {
		case a := <-l.C: /*blocking on lambda facing channel*/
			// check lambda status first
			t := time.Now()
			l.AliveLock.Lock()
			if l.Alive == false {
				myPrint("Lambda 2 is not alive, need to activate")
				l.Alive = true
				// trigger lambda
				go lambdaTrigger(l)
			}
			l.AliveLock.Unlock()
			fmt.Println("check lambda alive time is ", time.Since(t))
			//*
			// req from client
			//*
			// get channel and chunk id
			clientId := strconv.Itoa(a.Id.ClientId)
			reqId := strconv.Itoa(a.Id.ReqId)
			chunkId := strconv.Itoa(a.Id.ChunkId)
			//fmt.Println("client, chunk, reqId", clientId, chunkId, reqId)
			// get cmd argument
			cmd := strings.ToLower(a.Cmd)
			switch cmd {
			case "set": /*set or two argument cmd*/
				//myPrint("val is", a.Val, "id is ", clientId, "obj length is ", len(a.Val))
				// record the memory usage
				t := time.Now()
				l.Counter = l.Counter + uint64(len(a.Val))
				// write key and val in []byte format
				l.W.MyWriteCmd(a.Cmd, clientId, reqId, chunkId, a.Key, a.Val)
				err := l.W.Flush()
				if err != nil {
					fmt.Println("flush pipeline err is ", err)
				}
				fmt.Println("write to lambda store time is ", time.Since(t))
			case "get": /*get or one argument cmd*/
				t := time.Now()
				l.W.MyWriteCmd(a.Cmd, clientId, reqId, chunkId, a.Key)
				err := l.W.Flush()
				if err != nil {
					fmt.Println("flush pipeline err is ", err)
				}
				fmt.Println("write to lambda store time is ", time.Since(t))
			}
		case obj := <-l.Peek: /*blocking on lambda facing receive*/
			fmt.Println("aaaaaabbbbbbcccc")
			//group, ok := mappingTable.Get(obj.Key)
			group, ok := mappingTable.Get(0)
			if ok == false {
				fmt.Println("get lambda instance failed")
				return
			}
			// send chunk to group channel
			group.(*redeo.Group).C <- obj
		}
	}
}

func lambdaTrigger(l *redeo.LambdaInstance) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	client := lambda.New(sess, &aws.Config{Region: aws.String("us-east-1")})

	_, err := client.Invoke(&lambda.InvokeInput{FunctionName: aws.String(l.Name)})
	if err != nil {
		fmt.Println("Error calling LambdaFunction", err)
	}

	myPrint("Lambda Deactivate")
	l.AliveLock.Lock()
	l.Alive = false
	l.AliveLock.Unlock()
}

func myPrint(a ...interface{}) {
	if *isPrint {
		fmt.Println(a)
	}
}

func isFull(slice [][]byte) bool {
	isFull := true
	for i := range slice {
		if len(slice[i]) == 0 {
			isFull = false
		}
	}
	return isFull
}
