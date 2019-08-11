package lambdastore

import (
	"fmt"
	"encoding/json"
	"github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/wangaoone/LambdaObjectstore/lib/logger"
	"github.com/wangaoone/redeo"
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
		chanReq:   make(chan *redeo.ServerReq, 1024*1024),
		validated: validated,	// Initialize with a closed channel.
		log:       &logger.ColorLogger{
			Prefix: fmt.Sprintf("%s ", name),
			Level: global.Log.GetLevel(),
			Color: true,
		},
	}
}

func (ins *Instance) C() chan *redeo.ServerReq {
	return ins.chanReq
}

func (ins *Instance) Ping() {
	ins.cn.Ping()
}

func (ins *Instance) Validate() bool {
	select {
	case <-ins.validated:
		// Not validating. Validate...
		ins.log.Info("Validating...")
		ins.validated = make(chan bool)

		triggered := ins.alive == false && ins.tryTriggerLambda()
 		if !triggered {
			ins.Ping()
		}

		<-ins.validated
		return triggered
	default:
		// Validating... Wait and return false
		<-ins.validated
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
		select {
		case <-ins.validated:
			ins.alive = false
			return
		default:
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
