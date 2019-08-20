package lambdastore

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/wangaoone/LambdaObjectstore/lib/logger"
	"github.com/wangaoone/LambdaObjectstore/src/proxy/collector"
	"strings"
	"sync"
	"time"

	"github.com/wangaoone/LambdaObjectstore/src/proxy/global"
	"github.com/wangaoone/LambdaObjectstore/src/proxy/types"
	prototol "github.com/wangaoone/LambdaObjectstore/src/types"
)

type Instance struct {
	Name string
	Id   uint64

	replica   bool
	cn        *Connection
	chanReq   chan *types.Request
	chanWait  chan *types.Request
	alive     bool
	aliveLock sync.Mutex
	validated chan bool
	log       logger.ILogger
	mu        sync.Mutex
	closed    chan struct{}
}

// create new lambda instance
func NewInstance(name string, id uint64, replica bool) *Instance {
	if !replica {
		name = fmt.Sprintf("%s%d", name, id)
	}
	validated := make(chan bool)
	close(validated)

	return &Instance{
		Name:      name,
		Id:        id,
		replica:   replica,
		alive:     false,
		chanReq:   make(chan *types.Request, 1),
		chanWait:  make(chan *types.Request, 10),
		validated: validated, // Initialize with a closed channel.
		log: &logger.ColorLogger{
			Prefix: fmt.Sprintf("%s ", name),
			Level:  global.Log.GetLevel(),
			Color:  true,
		},
		closed: make(chan struct{}),
	}
}

func (ins *Instance) C() chan *types.Request {
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
	defer ins.mu.Unlock()

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
			// Check lambda status first
			validateStart := time.Now()
			ins.Validate()
			validateDuration := time.Since(validateStart)
			ins.log.Debug("validateDuration is %v", validateDuration)

			select {
			case <-ins.closed:
				// Again, check if instance is closed.
				return
			default:
			}

			cmd := strings.ToLower(req.Cmd)
			isDataRequest = false
			if cmd != "data" {
				if err := collector.Collect(collector.LogValidate, cmd, req.Id.ReqId, req.Id.ChunkId, int64(validateDuration)); err != nil {
					ins.log.Warn("Fail to record validate duration: %v", err)
				}
			}
			switch cmd {
			case "set": /*set or two argument cmd*/
				req.PrepareForSet(ins.cn.w)
			case "get": /*get or one argument cmd*/
				req.PrepareForGet(ins.cn.w)
			case "data":
				req.PrepareForData(ins.cn.w)
				isDataRequest = true
			}
			if err := req.Flush(); err != nil {
				ins.log.Error("Flush pipeline error: %v", err)
				if isDataRequest {
					global.DataCollected.Done()
				}
			}
			if !isDataRequest {
				ins.chanWait <- req
			}
		}
	}
}

func (ins *Instance) SetResponse(rsp *types.Response) {
	for req := range ins.chanWait {
		if req.IsResponse(rsp) {
			ins.log.Debug("response matched: %v", req.Id)
			req.ChanResponse <- rsp
			return
		}
		ins.log.Debug("passing req: %v, got %v", req, rsp)
	}
	ins.log.Error("Unexpected response: %v", rsp)
}

func (ins *Instance) SetErrorResponse(err error) {
	for req := range ins.chanWait {
		req.ChanResponse <- err
		return
	}
	ins.log.Error("Unexpected error response: %v", err)
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
	ins.flagValidatedLocked(true)
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
	event := &prototol.InputEvent{
		Id: ins.Id,
	}
	payload, _ := json.Marshal(event)
	input := &lambda.InvokeInput{
		FunctionName: aws.String(ins.Name),
		Payload:      payload,
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

	ins.flagValidatedLocked(false)
}

func (ins *Instance) flagValidatedLocked(onClose bool) {
	select {
	case <-ins.validated:
		// Validated
	default:
		if !onClose {
			ins.log.Info("Validated")
		}
		close(ins.validated)
	}
}
