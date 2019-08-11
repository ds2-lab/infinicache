package proxy

import (
	"github.com/wangaoone/LambdaObjectstore/lib/logger"
	"github.com/wangaoone/redeo"
	"net"

	"github.com/wangaoone/LambdaObjectstore/src/proxy/global"
	"github.com/wangaoone/LambdaObjectstore/src/proxy/lambdastore"
)

const NumLambdaClusters = 14
const LambdaStoreName = "LambdaStore"
const LambdaPrefix = "Proxy1Node"

type Proxy struct {
	log logger.ILogger
}

// initial lambda group
func New(replica bool) *Proxy {
	p := &Proxy{
		log:       &logger.ColorLogger{
			Prefix: "Proxy ",
			Level: global.Log.GetLevel(),
			Color: true,
		},
	}

	group := &redeo.Group{
		All: make([]redeo.LambdaInstance, NumLambdaClusters),
		MemCounter: 0,
	}
	global.Stores = group

	for i := range group.All {
		name := LambdaPrefix
		if replica {
			p.log.Info("[Registering lambda store replica %d.]", i)
			name = LambdaStoreName
		} else {
			p.log.Info("[Registering lambda store %s%d]", name, i)
		}
		node := lambdastore.NewInstance(name, uint64(i), replica)
		// register lambda instance to group
		group.All[i] = node

		go node.Validate()
	}

	return p
}

func (p *Proxy) Serve(lis net.Listener) {
	for {
		cn, err := lis.Accept()
		if err != nil {
			return
		}

		conn := lambdastore.NewConnection(cn)
		go conn.ServeLambda()
	}
}

func (p *Proxy) Close(lis net.Listener) {
	lis.Close()
}
