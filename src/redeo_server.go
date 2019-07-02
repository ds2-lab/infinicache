package main

import (
	"bytes"
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
)

var (
	clientLis    net.Listener
	lambdaLis    net.Listener
	cMap         = make(map[int]chan interface{}) // client channel mapping table
	mappingTable = hashmap.New(1024)              // lambda store mapping table
	shard        = 13
	isPrint      = true
)

func main() {
	clientLis, _ = net.Listen("tcp", ":6378")
	lambdaLis, _ = net.Listen("tcp", ":6379")
	fmt.Println("start listening client face port 6378")
	fmt.Println("start listening lambda face port 6379")
	srv := redeo.NewServer(nil)
	lambdaSrv := redeo.NewServer(nil)

	// initial lambda store
	initial(lambdaSrv)

	// lambda handler
	//go lambdaHandler(lambdaStore)
	// lambda facing peeking response type
	//go myPeek(lambdaStore)

	// initial ec2 server and lambda store

	// Start serving (blocking)
	err := srv.MyServe(clientLis, cMap, mappingTable)
	if err != nil {
		fmt.Println(err)
	}
}

func decoding(data [][]byte) []byte {
	enc, err := reedsolomon.New(10, 3)
	if err != nil {
		fmt.Println(err)
	}
	ok, err := enc.Verify(data)
	if ok {
		fmt.Println("No reconstruction needed")
	} else {
		fmt.Println("Verification failed. Reconstructing data")
		err = enc.Reconstruct(data)
		if err != nil {
			fmt.Println("Reconstruct failed -", err)
		}
		ok, err = enc.Verify(data)
		if !ok {
			fmt.Println("Verification failed after reconstruction, data likely corrupted.")
		}
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(ok)
	}
	var res bytes.Buffer
	err = enc.Join(&res, data, shard)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(res.Bytes())
	return res.Bytes()
}

// initial lambda group
func initial(lambdaSrv *redeo.Server) {
	group := redeo.Group{Arr: make([]redeo.LambdaInstance, shard), ChunkTable: make(map[string][][]byte), C: make(chan redeo.Response, 1024*1024)}
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
		go myPeek(node)
		myPrint(node.Alive)
	}
	mappingTable.Set(0, group)
	go groupReclaim(group)
}

func add(arr [][]byte, ele *[]byte) [][]byte {
	arr = append(arr, nil)
	arr[len(arr)-1] = *ele
	return arr
}

func groupReclaim(group redeo.Group) {
	for {
		// blocking on group channel
		obj := <-group.C
		// store obj body
		if len(group.ChunkTable[obj.Key]) != shard {
			group.ChunkTable[obj.Key] = add(group.ChunkTable[obj.Key], &obj.Body)
			//group.ChunkTable[obj.Key][obj.ChunkId] = obj.Body
		}
		fmt.Println("table is ", group.ChunkTable[obj.Key])
		// already get full response
		if len(group.ChunkTable[obj.Key]) == shard {
			clientId, _ := strconv.Atoi(obj.ClientId)
			myPrint("client id is ", clientId)
			res := decoding(group.ChunkTable[obj.Key])
			fmt.Println(res)
			// send response body to client channel
			cMap[clientId] <- "1"
			myPrint("lambda group has finished receive response from", shard, "lambdas")
			myPrint(group.ChunkTable[obj.Key])
			// release the map mem
			delete(group.ChunkTable, obj.Key)
		}
	}
}

func newLambdaInstance(name string) *redeo.LambdaInstance {
	return &redeo.LambdaInstance{
		//name:  "dataNode" + strconv.Itoa(id),
		Name:  name,
		Alive: false,
		C:     make(chan redeo.Req, 1024*1024),
		Peek:  make(chan redeo.Response, 1024*1024),
	}
}

// blocking on peekType, every response's type is bulk
func myPeek(l *redeo.LambdaInstance) {
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
		//
		// field 1 for client id
		// bulkString
		field1, err := l.R.PeekType()
		if err != nil {
			fmt.Println("field1 err", err)
			return
		}
		switch field1 {
		case resp.TypeBulk:
			obj.ClientId, _ = l.R.ReadBulkString()
		case resp.TypeError:
			err, _ := l.R.ReadError()
			fmt.Println("peek type err1 is", err)
		default:
			panic("unexpected response type")
		}
		//
		// field 2 for chunk id
		// Int
		field2, err := l.R.PeekType()
		if err != nil {
			fmt.Println("field2 err", err)
			return
		}
		switch field2 {
		case resp.TypeInt:
			obj.ChunkId, _ = l.R.ReadInt()
		case resp.TypeError:
			err, _ := l.R.ReadError()
			fmt.Println("peek type err2 is", err)
		default:
			panic("unexpected response type")
		}
		//
		// field 3 for obj body
		// bulkString
		field3, err := l.R.PeekType()
		if err != nil {
			fmt.Println("field3 err", err)
			return
		}
		switch field3 {
		case resp.TypeBulk:
			obj.Body, _ = l.R.ReadBulk(nil)
		case resp.TypeError:
			err, _ := l.R.ReadError()
			fmt.Println("peek type err3 is", err)
			return
		default:
			panic("unexpected response type")
		}
		// send obj to lambda helper channel
		l.Peek <- obj
	}
}

func lambdaHandler(l *redeo.LambdaInstance) {
	fmt.Println("conn is", l.Cn)
	for {
		select {
		case a := <-l.C: /*blocking on lambda facing channel*/
			// check lambda status first
			l.AliveLock.Lock()
			if l.Alive == false {
				myPrint("Lambda 2 is not alive, need to activate")
				l.Alive = true
				// trigger lambda
				go lambdaTrigger(l)
			}
			l.AliveLock.Unlock()
			//*
			// req from client
			//*
			// get channel and chunk id
			clientId := strconv.Itoa(a.ClientId)
			chunkId := strconv.Itoa(a.ChunkId)
			fmt.Println("client chunk", clientId, chunkId)
			// get cmd argument
			cmd := strings.ToLower(a.Cmd)
			switch cmd {
			case "get": /*get or one argument cmd*/
				l.W.MyWriteCmd(a.Cmd, clientId, chunkId, a.Key)
				err := l.W.Flush()
				if err != nil {
					fmt.Println("flush pipeline err is ", err)
				}
			case "set": /*set or two argument cmd*/
				myPrint("val is", a.Val, "id is ", clientId, "obj length is ", len(a.Val))
				// write key and val in []byte format
				l.W.MyWriteCmd(a.Cmd, clientId, chunkId, a.Key, a.Val)
				err := l.W.Flush()
				if err != nil {
					fmt.Println("flush pipeline err is ", err)
				}
			}
		case obj := <-l.Peek: /*blocking on lambda facing receive*/
			//group, ok := mappingTable.Get(obj.Key)
			group, ok := mappingTable.Get(0)
			if ok == false {
				fmt.Println("get lambda instance failed")
				return
			}
			// send chunk to group channel
			group.(redeo.Group).C <- obj
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
	if isPrint == true {
		fmt.Println(a)
	}
}
