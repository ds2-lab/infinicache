package lambdastore

import (
	_ "errors"
	_ "flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/wangaoone/LambdaObjectstore/lib/logger"
	"github.com/wangaoone/redeo"
	"github.com/wangaoone/redeo/resp"
	"net"
	_ "os"
	_ "os/signal"
	_ "strconv"
	_ "strings"
	"sync"
	_ "sync/atomic"
	_ "syscall"
	_ "time"
)

type Instance struct {
	Name         string
	Id           int
	Alive        bool
	Cn           net.Conn
	W            *resp.RequestWriter
	R            resp.ResponseReader
	chanReq      chan *redeo.ServerReq
	AliveLock    sync.Mutex
	Counter      uint64
	Closed       bool
	Validated    chan bool

	log          logger.ILogger
}

// create new lambda instance
func NewInstance(name string) *Instance {
	validated := make(chan bool)
	close(validated)

	return &Instance{
		Name:  name,
		Alive: false,
		chanReq:   make(chan *redeo.ServerReq, 1024*1024),
		Validated: validated,	// Initialize with a closed channel.
		log:       &logger.ColorLogger{
			Prefix: fmt.Sprintf("%s ", name),
			Color: true,
		},
	}
}

func (ins *Instance) C() chan *redeo.ServerReq {
	return ins.chanReq
}

func (ins *Instance) SetLogLevel(lvl int) {
	ins.log.(*logger.ColorLogger).Level = lvl
}

func (ins *Instance) Ping() {
	ins.W.WriteCmdString("ping")
	err := ins.W.Flush()
	if err != nil {
		ins.log.Error("Flush pipeline error(ping): %v", err)
	}
}

func (ins *Instance) Validate() bool {
	select {
	case <-ins.Validated:
		// Not validating. Validate...
		ins.Validated = make(chan bool)

		triggered := ins.Alive == false && ins.tryTriggerLambda()
 		if !triggered {
			ins.Ping()
		}

		<-ins.Validated
		return triggered
	default:
		// Validating... Wait and return false
		<-ins.Validated
		return false
	}
}

func (ins *Instance) tryTriggerLambda() bool {
	ins.AliveLock.Lock()
	defer ins.AliveLock.Unlock()

	if ins.Alive == true {
		return false
	}

	ins.log.Info("[Lambda store is not alive, activating...]")
	ins.Alive = true
	go ins.triggerLambda()

	return true
}

func (ins *Instance) triggerLambda() {
	ins.AliveLock.Lock()
	defer ins.AliveLock.Unlock()

	ins.triggerLambdaLocked()
	for {
		select {
		case <-ins.Validated:
			ins.Alive = false
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

	_, err := client.Invoke(&lambda.InvokeInput{FunctionName: aws.String(ins.Name)})
	if err != nil {
		ins.log.Error("Error on activating lambda store: %v", err)
	} else {
		ins.log.Info("[Lambda store is deactivated]")
	}
}
