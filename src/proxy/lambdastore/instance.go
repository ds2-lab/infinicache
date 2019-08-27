package lambdastore

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/wangaoone/LambdaObjectstore/src/proxy/collector"
	"github.com/wangaoone/LambdaObjectstore/lib/logger"
	"strings"
	"sync"
	"time"

	"github.com/wangaoone/LambdaObjectstore/src/proxy/global"
	"github.com/wangaoone/LambdaObjectstore/src/proxy/types"
	prototol "github.com/wangaoone/LambdaObjectstore/src/types"
)

var (
	Registry InstanceRegistry
)

type InstanceRegistry interface {
	Instance(uint64) (*Instance, bool)
}

type Instance struct {
	*Deployment

	cn      *Connection
	chanReq   chan interface{}
	chanWait  chan *types.Request
	alive     bool
	aliveLock sync.Mutex
	validated chan bool
	mu        sync.Mutex
	closed    chan struct{}
}

func NewInstanceFromDeployment(dp *Deployment) *Instance {
	dp.log = &logger.ColorLogger{
		Prefix: fmt.Sprintf("%s ", dp.name),
		Level:  global.Log.GetLevel(),
		Color:  true,
	}

	validated := make(chan bool)
	close(validated)

	return &Instance{
		Deployment: dp,
		alive:   false,
		chanReq:   make(chan interface{}, 1),
		chanWait:  make(chan *types.Request, 10),
		validated: validated, // Initialize with a closed channel.
		closed: make(chan struct{}),
	}
}

// create new lambda instance
func NewInstance(name string, id uint64, replica bool) *Instance {
	return NewInstanceFromDeployment(NewDeployment(name, id, replica))
}

func (ins *Instance) C() chan interface{} {
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
// lambda facing goroutine
func (ins *Instance) HandleRequests() {
	For:
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

			switch req.(type) {
			case types.Request:
				req := req.(types.Request)
				cmd := strings.ToLower(req.Cmd)

				if err := collector.Collect(collector.LogValidate, cmd, req.Id.ReqId, req.Id.ChunkId, int64(validateDuration)); err != nil {
					ins.log.Warn("Fail to record validate duration: %v", err)
				}

				switch cmd {
				case "set": /*set or two argument cmd*/
					req.PrepareForSet(ins.cn.w)
				case "get": /*get or one argument cmd*/
					req.PrepareForGet(ins.cn.w)
				default:
					ins.log.Error("Unexpected request command: %s", cmd)
					continue For
				}

				if err := req.Flush(); err != nil {
					ins.log.Error("Flush pipeline error: %v", err)
				}
				ins.chanWait <- &req

			case types.Control:
				ctrl := req.(types.Control)
				cmd := strings.ToLower(ctrl.Cmd)
				isDataRequest := false

				switch cmd {
				case "data":
					ctrl.PrepareForData(ins.cn.w)
					isDataRequest = true
				case "backup":
					ctrl.PrepareForBackup(ins.cn.w)
				default:
					ins.log.Error("Unexpected control command: %s", cmd)
					continue For
				}

				if err := ctrl.Flush(); err != nil {
					ins.log.Error("Flush pipeline error: %v", err)
					if isDataRequest {
						global.DataCollected.Done()
					}
				}
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

func (ins *Instance) Switch(to types.LambdaDeployment) *Instance {
	temp := &Deployment{}
	ins.Reset(to, temp)
	to.Reset(temp, nil)
	return ins
}

func (ins *Instance) Migrate() error {
	// func launch Mproxy
	// get addr if Mproxy
	dply, err := global.Migrator.GetDestination(ins.Id())
	if err != nil {
		ins.log.Error("Failed to find a backup destination: %v", err)
		return err
	}

	addr, err := global.Migrator.StartMigrator(ins.Id())
	if err != nil {
		ins.log.Error("Failed to start a migrator for backup: %v", err)
		return err
	}
	// expand local address
	if addr[0] == ':' {
		addr = global.ServerIp + addr
	}

	ins.chanReq <- &types.Control{
		Cmd: "migrate",
		Addr: addr,
		Deployment: dply.Name(),
		Id: dply.Id(),
	}
	return nil
}

func (ins *Instance) Close() {
	ins.mu.Lock()
	defer ins.mu.Unlock()

	if ins.isClosedLocked() {
		return
	}

	if ins.cn != nil {
		ins.cn.Close()
	}
	close(ins.closed)
	ins.flagValidatedLocked(true)
}

func (ins *Instance) IsClosed() bool {
	ins.mu.Lock()
	defer ins.mu.Unlock()

	return ins.isClosedLocked()
}

func (ins *Instance) isClosedLocked() bool {
	select {
	case <-ins.closed:
		// already closed
		return true
	default:
		return false
	}
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
		Id: ins.Id(),
		Proxy: fmt.Sprintf("%s:%d", global.ServerIp, global.BasePort + 1),
	}
	payload, _ := json.Marshal(event)
	input := &lambda.InvokeInput{
		FunctionName: aws.String(ins.Name()),
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
