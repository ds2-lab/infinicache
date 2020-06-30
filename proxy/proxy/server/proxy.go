package server

import (
	"github.com/google/uuid"

	"github.com/mason-leap-lab/redeo"
	"github.com/mason-leap-lab/redeo/resp"
	//	"github.com/google/uuid"
	"github.com/neboduus/infinicache/proxy/common/logger"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/neboduus/infinicache/proxy/proxy/collector"
	"github.com/neboduus/infinicache/proxy/proxy/global"
	"github.com/neboduus/infinicache/proxy/proxy/lambdastore"
	"github.com/neboduus/infinicache/proxy/proxy/types"
)

type Proxy struct {
	log       logger.ILogger
	group     *Group
	metaStore *Placer

	initialized int32
	ready       chan struct{}
}

// initial lambda group
func New(replica bool) *Proxy {
	group := NewGroup(NumLambdaClusters)
	p := &Proxy{
		log: &logger.ColorLogger{
			Prefix: "Proxy ",
			Level:  global.Log.GetLevel(),
			Color:  true,
		},
		group:     group,
		metaStore: NewPlacer(NewMataStore(), group),
		ready:     make(chan struct{}),
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
		node.Meta.Capacity = InstanceCapacity
		node.Meta.IncreaseSize(InstanceOverhead)

		// Initialize instance, this is not neccessary if the start time of the instance is acceptable.
		go func() {
			node.WarmUp()
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

	// Get args
	key, _ := c.NextArg().String()
	dChunkId, _ := c.NextArg().Int()
	chunkId := strconv.FormatInt(dChunkId, 10)
	lambdaId, _ := c.NextArg().Int()
	randBase, _ := c.NextArg().Int()
	reqId, _ := c.NextArg().String()
	// _, _ = c.NextArg().Int()
	// _, _ = c.NextArg().Int()
	dataChunks, _ := c.NextArg().Int()
	parityChunks, _ := c.NextArg().Int()

	bodyStream, err := c.Next()
	if err != nil {
		p.log.Error("Error on get value reader: %v", err)
		return
	}
	bodyStream.(resp.Holdable).Hold()

	// Start counting time.
	if err := collector.Collect(collector.LogStart, "set", reqId, chunkId, time.Now().UnixNano()); err != nil {
		p.log.Warn("Fail to record start of request: %v", err)
	}

	// We don't use this for now
	// global.ReqMap.GetOrInsert(reqId, &types.ClientReqCounter{"set", int(dataChunks), int(parityChunks), 0})

	// Check if the chunk key(key + chunkId) exists, base of slice will only be calculated once.
	prepared := p.metaStore.NewMeta(
		key, int(randBase), int(dataChunks+parityChunks), int(dChunkId), int(lambdaId), bodyStream.Len())

	meta, _, postProcess := p.metaStore.GetOrInsert(key, prepared)
	if meta.Deleted {
		// Object may be evicted in somecase:
		// 1: Some chunks were set.
		// 2: Placer evicted this object (unlikely).
		// 3: We got evicted meta.
		p.log.Warn("KEY %s@%s not set to lambda store, may got evicted before all chunks are set.", chunkId, key)
		w.AppendErrorf("KEY %s@%s not set to lambda store, may got evicted before all chunks are set.", chunkId, key)
		w.Flush()
		return
	}
	if postProcess != nil {
		postProcess(p.dropEvicted)
	}
	chunkKey := meta.ChunkKey(int(dChunkId))
	lambdaDest := meta.Placement[dChunkId]

	// Send chunk to the corresponding lambda instance in group
	p.log.Debug("Requesting to set %s: %d", chunkKey, lambdaDest)
	p.group.Instance(lambdaDest).C() <- &types.Request{
		Id:           types.Id{connId, reqId, chunkId},
		Cmd:          strings.ToLower(c.Name),
		Key:          chunkKey,
		BodyStream:   bodyStream,
		ChanResponse: client.Responses(),
		EnableCollector: true,
	}
	// p.log.Debug("KEY is", key.String(), "IN SET UPDATE, reqId is", reqId, "connId is", connId, "chunkId is", chunkId, "lambdaStore Id is", lambdaId)
}

func (p *Proxy) HandleGet(w resp.ResponseWriter, c *resp.Command) {
	client := redeo.GetClient(c.Context())
	connId := int(client.ID())
	key := c.Arg(0).String()
	dChunkId, _ := c.Arg(1).Int()
	chunkId := strconv.FormatInt(dChunkId, 10)
	reqId := c.Arg(2).String()
	dataChunks, _ := c.Arg(3).Int()
	parityChunks, _ := c.Arg(4).Int()

	// Start couting time.
	if err := collector.Collect(collector.LogStart, "get", reqId, chunkId, time.Now().UnixNano()); err != nil {
		p.log.Warn("Fail to record start of request: %v", err)
	}

	global.ReqMap.GetOrInsert(reqId, &types.ClientReqCounter{"get", int(dataChunks), int(parityChunks), 0, 0})

	// key is "key"+"chunkId"
	meta, ok := p.metaStore.Get(key, int(dChunkId))
	if !ok || meta.Deleted {
		// Object may be deleted.
		p.log.Warn("KEY %s@%s not found in lambda store, please set first.", chunkId, key)
		w.AppendErrorf("KEY %s@%s not found in lambda store, please set first.", chunkId, key)
		w.Flush()
		return
	}
	chunkKey := meta.ChunkKey(int(dChunkId))
	lambdaDest := meta.Placement[dChunkId]

	// Send request to lambda channel
	p.log.Debug("Requesting to get %s: %d", chunkKey, lambdaDest)
	p.group.Instance(lambdaDest).C() <- &types.Request{
		Id:           types.Id{connId, reqId, chunkId},
		Cmd:          strings.ToLower(c.Name),
		Key:          chunkKey,
		ChanResponse: client.Responses(),
		EnableCollector: true,
	}
}

func (p *Proxy) HandleMkSet(w resp.ResponseWriter, c *resp.Command) {
	client := redeo.GetClient(c.Context())
	connId := int(client.ID())

	// Get args
	key := c.Arg(0).String()
	dChunkId, _ := c.Arg(1).Int()
	chunkId := strconv.FormatInt(dChunkId, 10)
	lambdaId, _ := c.Arg(2).Int()
	randBase, _ := c.Arg(3).Int()
	reqId := c.Arg(4).String()
	dataChunks, _ := c.Arg(5).Int()
	parityChunks, _ := c.Arg(6).Int()
	// pairsN, _ := c.Arg(7).Int()

	size := 0
	var lowLevelKeys []string
	var values [][]byte

	for i := 8; i < c.ArgN(); i=i+2 {
		lowLevelKey := c.Arg(i).String()
		lowLevelKeys = append(lowLevelKeys, lowLevelKey)
		value := c.Arg(i+1).Bytes()
		values = append(values, value)
		size += len(value)
	}

	// Start counting time.
	if err := collector.Collect(collector.LogStart, "set", reqId, chunkId, time.Now().UnixNano()); err != nil {
		p.log.Warn("Fail to record start of request: %v", err)
	}

	// We don't use this for now
	// global.ReqMap.GetOrInsert(reqId, &types.ClientReqCounter{"set", int(dataChunks), int(parityChunks), 0})

	// Check if the chunk key(key + chunkId) exists, base of slice will only be calculated once.
	prepared := p.metaStore.NewMeta(
		key, int(randBase), int(dataChunks+parityChunks), int(dChunkId), int(lambdaId), int64(size))

	meta, _, postProcess := p.metaStore.GetOrInsert(key, prepared)

	if meta.Deleted {
		// Object may be evicted in somecase:
		// 1: Some chunks were set.
		// 2: Placer evicted this object (unlikely).
		// 3: We got evicted meta.
		p.log.Warn("KEY %s@%s not set to lambda store, may got evicted before all chunks are set.", chunkId, key)
		w.AppendErrorf("KEY %s@%s not set to lambda store, may got evicted before all chunks are set.", chunkId, key)
		w.Flush()
		return
	}
	if postProcess != nil {
		postProcess(p.dropEvicted)
	}
	chunkKey := meta.ChunkKey(int(dChunkId))
	lambdaDest := meta.Placement[dChunkId]

	// Send chunk to the corresponding lambda instance in group
	p.log.Debug("Requesting to mkset %s and low-level-keys %v on Lambda %d", chunkKey, lowLevelKeys, lambdaDest)
	p.group.Instance(lambdaDest).C() <- &types.Request{
		Id:              types.Id{connId, reqId, chunkId},
		Cmd:             strings.ToLower(c.Name),
		Key:             chunkKey,
		LowLevelKeys:    lowLevelKeys,
		LowLevelValues:  values,
		ChanResponse:    client.Responses(),
		EnableCollector: true,
	}
	// p.log.Debug("KEY is", key.String(), "IN SET UPDATE, reqId is", reqId, "connId is", connId, "chunkId is", chunkId, "lambdaStore Id is", lambdaId)
}

func (p *Proxy) HandleMkGet(w resp.ResponseWriter, c *resp.Command) {
	client := redeo.GetClient(c.Context())
	connId := int(client.ID())
	key := c.Arg(0).String()
	dChunkId, _ := c.Arg(1).Int()
	chunkId := strconv.FormatInt(dChunkId, 10)
	reqId := c.Arg(2).String()
	dataChunks, _ := c.Arg(3).Int()
	parityChunks, _ := c.Arg(4).Int()
	lowLevelKeysN, _ := c.Arg(5).Int()
	var lowLevelKeys []string

	for i:=6; i<c.ArgN() ;i++{
		lowLevelKey := c.Arg(i).String()
		lowLevelKeys = append(lowLevelKeys, lowLevelKey)
	}

	// Start couting time.
	if err := collector.Collect(collector.LogStart, "get", reqId, chunkId, time.Now().UnixNano()); err != nil {
		p.log.Warn("Fail to record start of request: %v", err)
	}

	global.ReqMap.GetOrInsert(reqId, &types.ClientReqCounter{"mkget", int(dataChunks), int(parityChunks), 0, int(lowLevelKeysN)})

	// key is "key"+"chunkId"
	meta, ok := p.metaStore.Get(key, int(dChunkId))
	if !ok || meta.Deleted {
		// Object may be deleted.
		p.log.Warn("KEY %s@%s not found in lambda store, please set first.", chunkId, key)
		w.AppendErrorf("KEY %s@%s not found in lambda store, please set first.", chunkId, key)
		w.Flush()
		return
	}
	chunkKey := meta.ChunkKey(int(dChunkId))
	lambdaDest := meta.Placement[dChunkId]

	// Send request to lambda channel
	p.log.Debug("Requesting to mkget %s, <%v>: %d", chunkKey, lowLevelKeys, lambdaDest)
	p.group.Instance(lambdaDest).C() <- &types.Request{
		Id:              types.Id{connId, reqId, chunkId},
		Cmd:             strings.ToLower(c.Name),
		Key:             chunkKey,
		ChanResponse:    client.Responses(),
		EnableCollector: true,
		LowLevelKeys:    lowLevelKeys,
	}
}

func (p *Proxy) HandleCallback(w resp.ResponseWriter, r interface{}) {
	wrapper := r.(*types.ProxyResponse)
	switch rsp := wrapper.Response.(type) {
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
		if wrapper.Request.EnableCollector {
			err := collector.Collect(collector.LogServer2Client, rsp.Cmd, rsp.Id.ReqId, rsp.Id.ChunkId, int64(tgg.Sub(t)), int64(d1), int64(d2), tgg.UnixNano())
			if err != nil {
				p.log.Warn("LogServer2Client err %v", err)
			}
		}
	// Use more general way to deal error
	default:
		w.AppendErrorf("%v", rsp)
		w.Flush()
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

func (p *Proxy) dropEvicted(meta *Meta) {
	reqId := uuid.New().String()
	for i, lambdaId := range meta.Placement {
		instance := p.group.Instance(lambdaId)
		instance.C() <- &types.Request{
			Id:  types.Id{0, reqId, strconv.Itoa(i)},
			Cmd: "del",
			Key: meta.ChunkKey(i),
		}
	}
}
