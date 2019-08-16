package proxy

import (
	"github.com/cornelk/hashmap"
	"github.com/wangaoone/LambdaObjectstore/lib/logger"
	"github.com/wangaoone/redeo"
	"github.com/wangaoone/redeo/resp"
	"net"
	"time"

	"github.com/wangaoone/LambdaObjectstore/src/proxy/types"
	"github.com/wangaoone/LambdaObjectstore/src/proxy/global"
	"github.com/wangaoone/LambdaObjectstore/src/proxy/lambdastore"
	"github.com/wangaoone/LambdaObjectstore/src/proxy/collector"
)

const NumLambdaClusters = 14
const LambdaStoreName = "LambdaStore"
const LambdaPrefix = "Proxy1Node"

type Proxy struct {
	log       logger.ILogger
	group     *types.Group
	metaMap   *hashmap.HashMap
}

// initial lambda group
func New(replica bool) *Proxy {
	p := &Proxy{
		log: &logger.ColorLogger{
			Prefix: "Proxy ",
			Level: global.Log.GetLevel(),
			Color: true,
		},
		group: &types.Group{
			All: make([]types.LambdaInstance, NumLambdaClusters),
			MemCounter: 0,
		},
		metaMap: hashmap.New(1024),
	}

	global.Stores = p.group

	for i := range p.group.All {
		name := LambdaPrefix
		if replica {
			p.log.Info("[Registering lambda store replica %d.]", i)
			name = LambdaStoreName
		} else {
			p.log.Info("[Registering lambda store %s%d]", name, i)
		}
		node := lambdastore.NewInstance(name, uint64(i), replica)
		// register lambda instance to group
		p.group.All[i] = node

		// Initialize instance, this is not neccessary if the start time of the instance is acceptable.
		go node.Validate()

		// Begin handle requests
		go node.HandleRequests()
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

func (p *Proxy) Release() {
	for _, node := range p.group.All {
		node.(*lambdastore.Instance).Close()
	}
	global.Stores = nil
}

func (p *Proxy) HandleSet(w resp.ResponseWriter, c *resp.CommandStream) {
	client := redeo.GetClient(c.Context())
	connId := int(client.ID())
	key, _ := c.NextArg().Bytes()
	chunkId, _ := c.NextArg().String()
	lambdaId, _ := c.NextArg().Int()
	reqId, _ := c.NextArg().String()
	dataShards, _ := c.NextArg().Int()
	parityShards, _ := c.NextArg().Int()
	bodyStream, err := c.Next()
	if err != nil {
		p.log.Error("Error on get value reader: %v", err)
		return
	}

	// Start couting time.
	if err := collector.Collect(collector.LogStart, "set", reqId, chunkId, time.Now().UnixNano()); err != nil {
		p.log.Warn("Fail to record start of request: %v", err)
	}

	global.ReqMap.GetOrInsert(reqId, &types.ClientReqCounter{"set", int(dataShards), int(parityShards), 0})

	// Check if the chunk key(key + chunkId) exists
	chunkKey := string(key) + chunkId
	request := &types.Request{
		Id: types.Id{ connId, reqId, chunkId },
		Cmd: c.Name,
		Key: key,
		BodyStream: bodyStream,
		ChanResponse: client.Responses(),
	}
	if lambdaDest, ok := p.metaMap.Get(chunkKey); !ok {
		// Send chunk to the corresponding lambda instance in group
		p.group.All[lambdaId].C() <- request
		p.metaMap.Set(chunkKey, lambdaId)
		// p.log.Debug("KEY is", key.String(), "IN SET, reqId is", reqId, "connId is", connId, "chunkId is", chunkId, "lambdaStore Id is", lambdaId)
	} else {
		// Update the existed key on original lambda
		p.group.All[lambdaDest.(int64)].C() <- request
		// p.log.Debug("KEY is", key.String(), "IN SET UPDATE, reqId is", reqId, "connId is", connId, "chunkId is", chunkId, "lambdaStore Id is", lambdaId)
	}
}

func (p *Proxy) HandleGet(w resp.ResponseWriter, c *resp.Command) {
	client := redeo.GetClient(c.Context())
	connId := int(client.ID())
	key := c.Arg(0)
	chunkId := c.Arg(1).String()
	reqId := c.Arg(2).String()
	dataShards, _ := c.Arg(3).Int()
	parityShards, _ := c.Arg(4).Int()

	// Start couting time.
	if err := collector.Collect(collector.LogStart, "get", reqId, chunkId, time.Now().UnixNano()); err != nil {
		p.log.Warn("Fail to record start of request: %v", err)
	}

	global.ReqMap.GetOrInsert(reqId, &types.ClientReqCounter{"get", int(dataShards), int(parityShards), 0})

	// key is "key"+"chunkId"
	lambdaDest, ok := p.metaMap.Get(key.String() + chunkId)
	// p.log.Debug("KEY is", key.String(), "IN GET, reqId is", reqId, "connId is", connId, "chunkId is", chunkId, "lambdaStore Id is", lambdaDestination)
	if !ok {
		p.log.Warn("KEY %s(%s) not found in lambda store, please set first.", key.String(), chunkId)
		w.AppendErrorf("KEY %s(%s) not found in lambda store, please set first.", key.String(), chunkId)
		w.Flush()
		return
	}
	// Send request to lambda channel
	p.group.All[lambdaDest.(int64)].C() <- &types.Request{
		Id: types.Id{ connId, reqId, chunkId },
		Cmd: c.Name,
		Key: key,
		ChanResponse: client.Responses(),
	}
}

func (p *Proxy) HandleCallback(w resp.ResponseWriter, r interface{}) {
	switch rsp := r.(type) {
	case error:
		w.AppendError(rsp.Error())
		w.Flush()
	case *types.Response:
		t := time.Now()

		rsp.PrepareFor(w)
		d1 := time.Since(t)

		t2 := time.Now()
		// flush buffer, return on errors
		if err := rsp.Flush(); err != nil {
			p.log.Error("Error on flush response: %v", err)
			return
		}
		d2 := time.Since(t2)
		//p.log.Debug("Server AppendInt time is", time0,
		//	"AppendBulk time is", time1,
		//	"Server Flush time is", time2,
		//	"Chunk body len is ", len(rsp.Body))
		tgg := time.Now()
		if err := collector.Collect(collector.LogServer2Client, rsp.Cmd, rsp.Id.ReqId, rsp.Id.ChunkId, int64(tgg.Sub(t)), int64(d1), int64(d2), tgg.UnixNano()); err != nil {
			p.log.Warn("LogServer2Client err %v", err)
		}
	}
}
