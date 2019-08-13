package lambdastore

import (
	"fmt"
	"encoding/json"
	"github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/wangaoone/LambdaObjectstore/lib/logger"
	"github.com/wangaoone/redeo"
	"strconv"
	"strings"
	"sync"

	"github.com/wangaoone/LambdaObjectstore/src/types"
	"github.com/wangaoone/LambdaObjectstore/src/proxy/global"
)

type Instance struct {
	Name         string
	Id           uint64

	replica      bool
	cn           *Connection
	chanReq      chan *redeo.ServerReq
	alive        bool
	aliveLock    sync.Mutex
	validated    chan bool
	log          logger.ILogger
	mu           sync.Mutex
	closed       chan struct{}
}

// create new lambda instance
func NewInstance(name string, id uint64, replica bool) *Instance {
	if !replica {
		name = fmt.Sprintf("%s%d", name, id)
	}
	validated := make(chan bool)
	close(validated)

	return &Instance{
		Name: name,
		Id: id,
		replica: replica,
		alive: false,
		chanReq:   make(chan *redeo.ServerReq, 1024),
		validated: validated,	// Initialize with a closed channel.
		log:       &logger.ColorLogger{
			Prefix: fmt.Sprintf("%s ", name),
			Level: global.Log.GetLevel(),
			Color: true,
		},
		closed:    make(chan struct{}),
	}
}

func (ins *Instance) C() chan *redeo.ServerReq {
	return ins.chanReq
}

func (ins *Instance) Validate() bool {
	ins.mu.Lock()

	select {
	case <-ins.validated:
		// Not validating. Validate...
		ins.validated = make(chan bool)
		ins.mu.Unlock()

		ins.log.Info("Validating...")
		triggered := ins.alive == false && ins.tryTriggerLambda()
 		if !triggered {
			ins.cn.Ping()
		}

		<-ins.validated
		return triggered
	default:
		// Validating... Wait and return false
		ins.mu.Unlock()
		<-ins.validated
		return false
	}
}

func (ins *Instance) IsValidating() bool {
	ins.mu.Lock()
	defer ins.mu.Lock()

	select {
	case <-ins.validated:
		return false
	default:
		return true
	}
}

// Handle incoming client requests
func (ins *Instance) HandleRequests() {
	var isDataRequest bool
	for {
		select {
		case <-ins.closed:
			return
		case req := <-ins.chanReq: /*blocking on lambda facing channel*/
			// check lambda status first
			ins.Validate()

			// get arguments
			connId := strconv.Itoa(req.Id.ConnId)
			chunkId := strconv.FormatInt(req.Id.ChunkId, 10)

			cmd := strings.ToLower(req.Cmd)
			isDataRequest = false
			switch cmd {
			case "set": /*set or two argument cmd*/
				ins.cn.w.MyWriteCmd(req.Cmd, connId, req.Id.ReqId, chunkId, req.Key, req.Body)
			case "get": /*get or one argument cmd*/
				ins.cn.w.MyWriteCmd(req.Cmd, connId, req.Id.ReqId, "", req.Key)
			case "data":
				ins.cn.w.WriteCmdString(req.Cmd)
				isDataRequest = true
			}
			err := ins.cn.w.Flush()
			if err != nil {
				ins.log.Error("Flush pipeline error: %v", err)
				if isDataRequest {
					global.DataCollected.Done()
				}
			}
		}
	}
}

func (ins *Instance) Close() {
	ins.mu.Lock()
	defer ins.mu.Unlock()

	select {
	case <-ins.closed:
		// already closed
		return
	default:
	}

	if ins.cn != nil {
		ins.cn.Close()
	}
	close(ins.closed)
}

func (ins *Instance) tryTriggerLambda() bool {
	ins.aliveLock.Lock()
	defer ins.aliveLock.Unlock()

	if ins.alive == true {
		return false
	}

	ins.log.Info("[Lambda store is not alive, activating...]")
	ins.alive = true
	go ins.triggerLambda()

	return true
}

func (ins *Instance) triggerLambda() {
	ins.aliveLock.Lock()
	defer ins.aliveLock.Unlock()

	ins.triggerLambdaLocked()
	for {
		if !ins.IsValidating() {
			ins.alive = false
			return
		}

		// Validating, retrigger.
		ins.log.Info("[Validating lambda store,  reactivateing...]")
		ins.triggerLambdaLocked()
	}
}

func (ins *Instance) triggerLambdaLocked() {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	client := lambda.New(sess, &aws.Config{Region: aws.String("us-east-1")})
	event := &types.InputEvent {
		Id: ins.Id,
	}
	payload, _ := json.Marshal(event)
	input := &lambda.InvokeInput{
		FunctionName: aws.String(ins.Name),
		Payload: payload,
	}

	_, err := client.Invoke(input)
	if err != nil {
		ins.log.Error("Error on activating lambda store: %v", err)
	} else {
		ins.log.Info("[Lambda store is deactivated]")
	}
}

func (ins *Instance) flagValidated(conn *Connection) {
	ins.mu.Lock()
	defer ins.mu.Unlock()

	if ins.cn != conn {
		// Set instance, order matters here.
		conn.instance = ins
		conn.log = ins.log
		if ins.cn != nil {
			ins.cn.Close()
		}
		ins.cn = conn
	}

	select {
	case <-ins.validated:
		// Validated
	default:
		ins.log.Info("Validated")
		close(ins.validated)
	}
}
