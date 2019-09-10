package lambdastore

import (
	"encoding/json"
	"errors"
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
	MAX_RETRY = 3
)

var (
	Registry InstanceRegistry
	TimeoutNever = make(<-chan time.Time)
	WarmTimout = 1 * time.Minute
	ConnectTimeout = 20 * time.Millisecond // Just above average triggering cost.
	RequestTimeout = 30 * time.Second
)

type InstanceRegistry interface {
	Instance(uint64) (*Instance, bool)
}

type Instance struct {
	*Deployment

	cn        *Connection
	chanReq   chan interface{}
	alive     int
	aliveLock sync.Mutex
	chanValidated chan struct{}
	lastValidated *Connection
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

	chanValidated := make(chan struct{})
	close(chanValidated)

	return &Instance{
		Deployment: dp,
		alive:      INSTANCE_DEAD,
		chanReq:    make(chan interface{}, 1),
		chanValidated:  chanValidated, // Initialize with a closed channel.
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

func (ins *Instance) WarmUp() {
	ins.validate(true)
}

func (ins *Instance) Validate() *Connection {
	return ins.validate(false)
}

func (ins *Instance) validate(warmUp bool) *Connection {
	ins.mu.Lock()

	select {
	case <-ins.chanValidated:
		// Not validating. Validate...
		ins.chanValidated = make(chan struct{})
		ins.lastValidated = nil
		ins.mu.Unlock()

		for {
			ins.log.Debug("Validating...")
			triggered := ins.alive == INSTANCE_DEAD && ins.tryTriggerLambda(warmUp)
			if triggered {
				return ins.validated()
			} else if warmUp {
				return ins.flagValidatedLocked(ins.cn)
			}

			// ping is issued to ensure alive
			ins.cn.Ping()

			// If we are not sure instance status, set a short timeout and trigger after timeout.
			var timeout <-chan time.Time
			if ins.alive == INSTANCE_MAYBE {
				timer := time.NewTimer(ConnectTimeout)
				timeout = timer.C
			} else {
				timeout = TimeoutNever
			}

			select{
			case <-timeout:
				// Set status to dead and revalidate.
				ins.alive = INSTANCE_DEAD
				ins.log.Warn("Timeout on validating, assuming instance dead and retry...")
			case <-ins.chanValidated:
				return ins.validated()
			}
		}
	default:
		// Validating... Wait and return false
		ins.mu.Unlock()
		return ins.validated()
	}
}

func (ins *Instance) IsValidating() bool {
	ins.mu.Lock()
	defer ins.mu.Unlock()

	select {
	case <-ins.chanValidated:
		return false
	default:
		return true
	}
}

// Handle incoming client requests
// lambda facing goroutine
func (ins *Instance) HandleRequests() {
	for {
		select {
		case <-ins.closed:
			return
		case req := <-ins.chanReq: /*blocking on lambda facing channel*/
			var err error
			for i := 0; i < MAX_RETRY; i++ {
				if i > 0 {
					ins.log.Debug("Attempt %d", i)
				}
				// Check lambda status first
				validateStart := time.Now()
				// Once active connection is confirmed, keep alive on serving.
				conn := ins.Validate()
				validateDuration := time.Since(validateStart)

				if conn == nil {
					// Check if conn is valid, nil if ins get closed
					return
				}
				err = ins.handleRequest(conn, req, validateDuration)
				if err == nil {
					break
				}
			}
			if err != nil {
				ins.log.Error("Max retry reaches, give up")
				if request, ok := req.(*types.Request); ok {
					request.SetResponse(err)
				}
			}
			ins.warmUp()
		case <-ins.coolTimer.C:
			// Warm up
			ins.WarmUp()
		}
	}
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

	ins.log.Debug("Closing...")
	close(ins.closed)
	if !ins.coolTimer.Stop() {
		select {
		case <-ins.coolTimer.C:
		default:
		}
	}
	if ins.cn != nil {
		ins.cn.Close()
	}
	ins.flagValidatedLocked(nil)
}

func (ins *Instance) IsClosed() bool {
	ins.mu.Lock()
	defer ins.mu.Unlock()

	return ins.isClosedLocked()
}

func (ins *Instance) tryTriggerLambda(warmUp bool) bool {
	ins.aliveLock.Lock()
	defer ins.aliveLock.Unlock()

	if ins.alive == INSTANCE_ALIVE {
		return false
	}

	if warmUp {
		ins.log.Info("[Lambda store is not alive, warming up...]")
	} else {
		ins.log.Info("[Lambda store is not alive, activating...]")
	}
	go ins.triggerLambda(warmUp)

	return true
}

func (ins *Instance) triggerLambda(warmUp bool) {
	ins.triggerLambdaLocked(warmUp)
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
		ins.triggerLambdaLocked(false)
	}
}

func (ins *Instance) triggerLambdaLocked(warmUp bool) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	client := lambda.New(sess, &aws.Config{Region: aws.String("us-east-1")})
	event := &prototol.InputEvent{
		Id:     ins.Id(),
		Proxy:  fmt.Sprintf("%s:%d", global.ServerIp, global.BasePort+1),
		Prefix: global.Prefix,
		Log:    global.Log.GetLevel(),
	}
	if warmUp {
		event.Cmd = "warmup"
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

	ins.warmUp()
	if ins.cn != conn {
		oldConn := ins.cn

		// Set instance, order matters here.
		conn.instance = ins
		conn.log = ins.log
		ins.cn = conn

		if oldConn != nil {
			oldConn.Close()

			if oldConn.instance == ins {
				// There are two possibilities for connectio switch:
				// 1. Migration
				// 2. Accidential concurrent triggering, usually after lambda returning and before it get reclaimed.
				// In either case, the status is alive and it indicate the status of the old instance, it is not reliable.
				ins.aliveLock.Lock()
				defer ins.aliveLock.Unlock()
				ins.alive = INSTANCE_MAYBE
			}
		}
		// No need to set alive for new connection, it has been set already.
	} else {
		ins.alive = INSTANCE_ALIVE
	}

	ins.flagValidatedLocked(conn)
}

func (ins *Instance) bye(conn *Connection) {
	ins.mu.Lock()
	defer ins.mu.Unlock()

	if ins.cn == conn {
		ins.aliveLock.Lock()
		defer ins.aliveLock.Unlock()

		ins.alive = INSTANCE_DEAD
	}
}

func (ins *Instance) flagValidatedLocked(conn *Connection) *Connection {
	select {
	case <-ins.chanValidated:
		// Validated
	default:
		if conn != nil {
			ins.log.Debug("Validated")
			ins.lastValidated = conn
		}
		close(ins.chanValidated)
	}
	return ins.lastValidated
}

func (ins *Instance) validated() *Connection {
	<-ins.chanValidated
	return ins.lastValidated
}

func (ins *Instance) handleRequest(conn *Connection, req interface{}, validateDuration time.Duration) error {
	switch req.(type) {
	case *types.Request:
		req := req.(*types.Request)

		cmd := strings.ToLower(req.Cmd)
		if err := collector.Collect(collector.LogValidate, cmd, req.Id.ReqId, req.Id.ChunkId, int64(validateDuration)); err != nil {
			ins.log.Warn("Fail to record validate duration: %v", err)
		}

		switch cmd {
		case "set": /*set or two argument cmd*/
			req.PrepareForSet(conn.w)
		case "get": /*get or one argument cmd*/
			req.PrepareForGet(conn.w)
		default:
			req.SetResponse(errors.New(fmt.Sprintf("Unexpected request command: %s", cmd)))
			// Unrecoverable
			return nil
		}

		// In case there is a request already, wait to be consumed (for response).
		conn.chanWait <- req
		conn.cn.SetWriteDeadline(time.Now().Add(RequestTimeout))  // Set deadline for write
		defer conn.cn.SetWriteDeadline(time.Time{})
		if err := req.Flush(); err != nil {
			ins.log.Warn("Flush pipeline error: %v", err)
			// Remove request.
			select {
			case <-conn.chanWait:
			default:
			}
			return err
		}

	case *types.Control:
		ctrl := req.(*types.Control)
		cmd := strings.ToLower(ctrl.Cmd)
		isDataRequest := false

		switch cmd {
		case "data":
			ctrl.PrepareForData(conn.w)
			isDataRequest = true
		case "migrate":
			ctrl.PrepareForMigrate(conn.w)
		default:
			ins.log.Error("Unexpected control command: %s", cmd)
			// Unrecoverable
			return nil
		}

		if err := ctrl.Flush(); err != nil {
			ins.log.Error("Flush pipeline error: %v", err)
			if isDataRequest {
				global.DataCollected.Done()
			}
			// Control commands are valid to connection only.
			return nil
		}

	default:
		ins.log.Error("Unexpected request type: %v", reflect.TypeOf(req))
		// Unrecoverable
		return nil
	}

	return nil
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

func (ins *Instance) warmUp() {
	if !ins.coolTimer.Stop() {
		select{
		case <-ins.coolTimer.C:
		default:
		}
	}
	ins.coolTimer.Reset(WarmTimout)
}
