package proxy

import (
	"fmt"
	"github.com/cornelk/hashmap"
	"github.com/wangaoone/LambdaObjectstore/lib/logger"
	"github.com/wangaoone/redeo"
	"github.com/wangaoone/redeo/resp"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/wangaoone/LambdaObjectstore/src/proxy/collector"
	"github.com/wangaoone/LambdaObjectstore/src/proxy/global"
	"github.com/wangaoone/LambdaObjectstore/src/proxy/lambdastore"
	"github.com/wangaoone/LambdaObjectstore/src/proxy/types"
)

type Proxy struct {
	log     logger.ILogger
	group   *Group
	metaMap *hashmap.HashMap

	initialized int32
	ready       chan struct{}
}

// initial lambda group
func New(replica bool) *Proxy {
	p := &Proxy{
		log: &logger.ColorLogger{
			Prefix: "Proxy ",
			Level:  global.Log.GetLevel(),
			Color:  true,
		},
		group:   NewGroup(NumLambdaClusters),
		metaMap: hashmap.New(1024),
		ready:   make(chan struct{}),
	}

	for i := range p.group.All {
		name := LambdaPrefix
		if replica {
			p.log.Info("[Registering lambda store replica %d.]", i)
			name = LambdaStoreName
		} else {
			p.log.Info("[Registering lambda store %s%d]", name, i)
		}
		node := scheduler.GetForGroup(p.group, i)

		// Initialize instance, this is not neccessary if the start time of the instance is acceptable.
		go func() {
			node.Validate(true)
			if atomic.AddInt32(&p.initialized, 1) == int32(p.group.Len()) {
				p.log.Info("[Proxy is ready]")
				close(p.ready)
			}
		}()

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

func (p *Proxy) Ready() chan struct{} {
	return p.ready
}

func (p *Proxy) Close(lis net.Listener) {
	lis.Close()
}

func (p *Proxy) Release() {
	for i, node := range p.group.All {
		scheduler.Recycle(node.LambdaDeployment)
		p.group.All[i] = nil
	}
	scheduler.Clear(p.group)
}

// from client
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
	bodyStream.(resp.Holdable).Hold()

	// Start couting time.
	if err := collector.Collect(collector.LogStart, "set", reqId, chunkId, time.Now().UnixNano()); err != nil {
		p.log.Warn("Fail to record start of request: %v", err)
	}

	global.ReqMap.GetOrInsert(reqId, &types.ClientReqCounter{"set", int(dataShards), int(parityShards), 0})

	// Check if the chunk key(key + chunkId) exists
	chunkKey := fmt.Sprintf("%s@%s", chunkId, string(key))
	request := &types.Request{
		Id:           types.Id{connId, reqId, chunkId},
		Cmd:          strings.ToLower(c.Name),
		Key:          []byte(chunkKey),
		BodyStream:   bodyStream,
		ChanResponse: client.Responses(),
	}
	lambdaDest, _ := p.metaMap.GetOrInsert(chunkKey, int(lambdaId))
	// Send chunk to the corresponding lambda instance in group
	p.group.Instance(lambdaDest.(int)).C() <- request
	// p.log.Debug("KEY is", key.String(), "IN SET UPDATE, reqId is", reqId, "connId is", connId, "chunkId is", chunkId, "lambdaStore Id is", lambdaId)
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
	chunkKey := fmt.Sprintf("%s@%s", chunkId, string(key))
	lambdaDest, ok := p.metaMap.Get(chunkKey)
	// p.log.Debug("KEY is", key.String(), "IN GET, reqId is", reqId, "connId is", connId, "chunkId is", chunkId, "lambdaStore Id is", lambdaDestination)
	if !ok {
		p.log.Warn("KEY %s not found in lambda store, please set first.", chunkKey)
		w.AppendErrorf("KEY %s not found in lambda store, please set first.", chunkKey)
		w.Flush()
		return
	}
	// Send request to lambda channel
	p.group.Instance(lambdaDest.(int)).C() <- &types.Request{
		Id:           types.Id{connId, reqId, chunkId},
		Cmd:          strings.ToLower(c.Name),
		Key:          []byte(chunkKey),
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

func (p *Proxy) CollectData() {
	for i, _ := range p.group.All {
		global.DataCollected.Add(1)
		// send data command
		p.group.Instance(i).C() <- &types.Control{Cmd: "data"}
	}
	p.log.Info("Waiting data from Lambda")
	global.DataCollected.Wait()
	if err := collector.Flush(); err != nil {
		p.log.Error("Failed to save data from lambdas: %v", err)
	} else {
		p.log.Info("Data collected.")
	}
}
