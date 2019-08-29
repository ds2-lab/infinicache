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
	"reflect"

	"github.com/wangaoone/LambdaObjectstore/src/proxy/global"
	"github.com/wangaoone/LambdaObjectstore/src/proxy/types"
	prototol "github.com/wangaoone/LambdaObjectstore/src/types"
)

const (
	INSTANCE_DEAD = 0
	INSTANCE_ALIVE = 1
	INSTANCE_MAYBE = 2
)

var (
	Registry InstanceRegistry
	TimeoutNever = make(<-chan time.Time)
	WarmTimout = 1 * time.Minute
)

type InstanceRegistry interface {
	Instance(uint64) (*Instance, bool)
}

type Instance struct {
	*Deployment

	cn        *Connection
	chanReq   chan interface{}
	chanWait  chan *types.Request
	alive     int
	aliveLock sync.Mutex
	validated chan bool
	mu        sync.Mutex
	closed    chan struct{}
	coolTimer *time.Timer
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
		alive:      INSTANCE_DEAD,
		chanReq:    make(chan interface{}, 1),
		chanWait:   make(chan *types.Request, 10),
		validated:  validated, // Initialize with a closed channel.
		closed:     make(chan struct{}),
		coolTimer:  time.NewTimer(WarmTimout),
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

		ins.log.Debug("Validating...")
		triggered := ins.alive == INSTANCE_DEAD && ins.tryTriggerLambda()
		if triggered {
			<-ins.validated
			return triggered
		}

		// ping is issued to ensure alive
		ins.cn.Ping()

		// If we are not sure instance status, set a short timeout and trigger after timeout.
		var timeout <-chan time.Time
		if ins.alive == INSTANCE_MAYBE {
			timer := time.NewTimer(10 * time.Millisecond) // average triggering cost.
			timeout = timer.C
		} else {
			timeout = TimeoutNever
		}

		select{
		case <-timeout:
			// Set status to dead and revalidate.
			ins.log.Warn("Timeout on validating, assuming instance dead and retry...")
			ins.alive = INSTANCE_DEAD
			return ins.Validate()
		case <-ins.validated:
			return triggered
		}
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

			ins.warm()

			switch req.(type) {
			case *types.Request:
				req := req.(*types.Request)
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
				ins.chanWait <- req

			case *types.Control:
				ctrl := req.(*types.Control)
				cmd := strings.ToLower(ctrl.Cmd)
				isDataRequest := false

				switch cmd {
				case "data":
					ctrl.PrepareForData(ins.cn.w)
					isDataRequest = true
				case "migrate":
					ctrl.PrepareForMigrate(ins.cn.w)
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

			default:
				ins.log.Error("Unexpected request type: %v", reflect.TypeOf(req))
			}
		case <-ins.coolTimer.C:
			// Warm up
			ins.Validate()
			ins.warm()
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
		ins.log.Error("Failed to find a migration destination: %v", err)
		return err
	}

	addr, err := global.Migrator.StartMigrator(ins.Id())
	if err != nil {
		ins.log.Error("Failed to start a migrator: %v", err)
		return err
	}
	// expand local address
	if addr[0] == ':' {
		addr = global.ServerIp + addr
	}

	ins.log.Info("Initiating migration to %s...", dply.Name())
	ins.chanReq <- &types.Control{
		Cmd:        "migrate",
		Addr:       addr,
		Deployment: dply.Name(),
		Id:         dply.Id(),
	}
	return nil
}

func (ins *Instance) Close() {
	ins.mu.Lock()
	defer ins.mu.Unlock()

	if ins.isClosedLocked() {
		return
	}

	if !ins.coolTimer.Stop() {
		select {
		case <-ins.coolTimer.C:
		default:
		}
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

	if ins.alive == INSTANCE_ALIVE {
		return false
	}

	ins.log.Info("[Lambda store is not alive, activating...]")
	ins.alive = INSTANCE_ALIVE
	go ins.triggerLambda()

	return true
}

func (ins *Instance) triggerLambda() {
	ins.triggerLambdaLocked()
	for {
		if !ins.IsValidating() {
			// Don't overwrite the MAYBE status.
			ins.aliveLock.Lock()
			if ins.alive != INSTANCE_MAYBE {
				ins.alive = INSTANCE_DEAD
			}
			ins.aliveLock.Unlock()
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
		Id:     ins.Id(),
		Proxy:  fmt.Sprintf("%s:%d", global.ServerIp, global.BasePort+1),
		Prefix: global.Prefix,
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
		ins.log.Debug("[Lambda store is deactivated]")
	}
}

func (ins *Instance) flagValidated(conn *Connection) {
	ins.mu.Lock()
	defer ins.mu.Unlock()

	if ins.cn != conn {
		oldIns := conn.instance
		oldConn := ins.cn

		// Set instance, order matters here.
		conn.instance = ins
		conn.log = ins.log
		ins.cn = conn

		if oldConn != nil {
			oldConn.Close()
		}
		if oldIns != nil && oldIns.Id() != ins.Id() {
			// There are two possibilities for connectio switch:
			// 1. Migration
			// 2. Accidential concurrent triggering, usually after lambda returning and before it get reclaimed.
			// Here, we consider possibility 1 only.
			// TODO: deal with possibility 2
			ins.aliveLock.Lock()
			defer ins.aliveLock.Unlock()
			ins.alive = INSTANCE_MAYBE
		}
	}

	ins.flagValidatedLocked(false)
}

func (ins *Instance) flagValidatedLocked(onClose bool) {
	select {
	case <-ins.validated:
		// Validated
	default:
		if !onClose {
			ins.log.Debug("Validated")
		}
		close(ins.validated)
	}
}

func (ins *Instance) warm() {
	if !ins.coolTimer.Stop() {
		select{
		case <-ins.coolTimer.C:
		default:
		}
	}
	ins.coolTimer.Reset(WarmTimout)
}
