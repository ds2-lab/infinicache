package client

import (
	"errors"
	"fmt"
	"github.com/ScottMansfield/nanolog"
	"github.com/fatih/set"
	"github.com/google/uuid"
	"github.com/mason-leap-lab/redeo/resp"
	"io"
	"math/rand"
	"strconv"
	"sync"
	"time"
)

type KeyValuePair struct {
	Key   string
	Value []byte
}

type KVSetGroup struct {
	KeyValuePairs []KeyValuePair
}

type KVGetGroup struct {
	Keys []string
}

func (c *Client) MkSet(highLevelKey string, data [3]KVSetGroup, args ...interface{}) (string, bool) {
	// Debuging options
	var dryrun int
	var placements []int
	if len(args) > 0 {
		dryrun, _ = args[0].(int)
	}
	if len(args) > 1 {
		p, ok := args[1].([]int)
		if ok && len(p) >= c.Shards {
			placements = p
		}
	}

	stats := &c.Data
	stats.Begin = time.Now()
	stats.ReqId = uuid.New().String()

	// randomly generate destiny lambda store id
	numClusters := MaxLambdaStores
	if dryrun > 0 {
		numClusters = dryrun
	}
	index := random(numClusters, c.MKReplicationFactors[0])
	if dryrun > 0 && placements != nil {
		for i, ret := range index {
			placements[i] = ret
		}
		return stats.ReqId, true
	}

	//addr, ok := c.getHost(highLevelKey)
	//fmt.Println("in SET, highLevelKey is: ", highLevelKey)
	member := c.Ring.LocateKey([]byte(highLevelKey))
	host := member.String()
	log.Debug("ring LocateKey costs: %v", time.Since(stats.Begin))
	log.Debug("SET located host: %s", host)

	var wg sync.WaitGroup
	ret := newEcRet(c.MKReplicationFactors[0])
	var replicas = replicate(data, c.MKReplicationFactors[0])

	for i := 0; i < len(replicas); i++ {
		// fmt.Println("shards", i, "is", shards[i])
		wg.Add(1)
		go c.mkSet(host, highLevelKey, replicas[i], i, index[i], stats.ReqId, &wg, ret)
	}
	wg.Wait()
	stats.ReqLatency = time.Since(stats.Begin)
	stats.Duration = stats.ReqLatency

	if ret.Err != nil {
		return stats.ReqId, false
	}

	nanolog.Log(LogClient, "set", stats.ReqId, stats.Begin.UnixNano(),
		int64(stats.Duration), int64(stats.ReqLatency), int64(0), int64(0),
		false, false)
	log.Info("Set %s %d", highLevelKey, int64(stats.Duration))

	if placements != nil {
		for i, ret := range ret.Rets {
			placements[i], _ = strconv.Atoi(string(ret.([]byte)))
		}
	}

	return stats.ReqId, true
}

func (c *Client) MkGet(highLevelKey string, lowLevelKeys [3]KVGetGroup, args ...interface{}) (string, io.ReadCloser, bool) {
	var dryrun int
	if len(args) > 0 {
		dryrun, _ = args[0].(int)
	}

	stats := &c.Data
	stats.Begin = time.Now()
	stats.ReqId = uuid.New().String()
	if dryrun > 0 {
		return stats.ReqId, nil, true
	}

	//addr, ok := c.getHost(highLevelKey)
	member := c.Ring.LocateKey([]byte(highLevelKey))
	host := member.String()
	//fmt.Println("ring LocateKey costs:", time.Since(t))
	//fmt.Println("GET located host: ", host)

	// Send request and wait
	var wg sync.WaitGroup
	locations := locateLowLevelKeys(lowLevelKeys)
	ret := newEcRet(len(locations))
	for i, v := range locations {
		wg.Add(1)
		go c.mkGet(host, highLevelKey, i, v, stats.ReqId, &wg, ret)
	}
	wg.Wait()
	stats.RecLatency = time.Since(stats.Begin)

	// Filter results
	chunks := make([][]byte, ret.Len())
	failed := make([]int, 0, ret.Len())
	for i, _ := range ret.Rets {
		err := ret.Error(i)
		if err != nil {
			failed = append(failed, i)
		} else {
			chunks[i] = ret.Ret(i)
		}
	}

	decodeStart := time.Now()
	reader, err := c.decode(stats, chunks, 5)
	if err != nil {
		return stats.ReqId, nil, false
	}

	end := time.Now()
	stats.Duration = end.Sub(stats.Begin)
	nanolog.Log(LogClient, "get", stats.ReqId, stats.Begin.UnixNano(),
		int64(stats.Duration), int64(0), int64(stats.RecLatency), int64(end.Sub(decodeStart)),
		stats.AllGood, stats.Corrupted)
	log.Info("Got %s %d ( %d %d )", highLevelKey, int64(stats.Duration), int64(stats.RecLatency), int64(end.Sub(decodeStart)))

	// Try recover
	if len(failed) > 0 {
		c.recover(host, highLevelKey, uuid.New().String(), chunks, failed)
	}

	return stats.ReqId, reader, true
}

func (c *Client) mkSet(addr string, key string, replica KVSetGroup, i int, lambdaId int, reqId string, wg *sync.WaitGroup, ret *ecRet) {
	defer wg.Done()

	if err := c.validate(addr, i); err != nil {
		c.setError(ret, addr, i, err)
		log.Warn("Failed to validate connection %d@%s(%s): %v", i, key, addr, err)
		return
	}
	cn := c.Conns[addr][i]
	cn.conn.SetWriteDeadline(time.Now().Add(Timeout)) // Set deadline for request
	defer cn.conn.SetWriteDeadline(time.Time{})

	var pairs = len(replica.KeyValuePairs)

	fmt.Println("mkSetting replica, ", replica)

	w := cn.W
	w.WriteMultiBulkSize(9+(2*pairs))
	w.WriteBulkString("mkset")
	w.WriteBulkString(key)
	w.WriteBulkString(strconv.Itoa(i))
	w.WriteBulkString(strconv.Itoa(lambdaId))
	w.WriteBulkString(strconv.Itoa(MaxLambdaStores))
	w.WriteBulkString(reqId)
	w.WriteBulkString(strconv.Itoa(c.MKReplicationFactors[0]))
	w.WriteBulkString(strconv.Itoa(0))
	w.WriteBulkString(strconv.Itoa(pairs))

	for i := 0; i < pairs; i++ {
		var pair = replica.KeyValuePairs[i]
		// Flush pipeline
		//if err := c.W[i].Flush(); err != nil {
		w.WriteBulkString(pair.Key)
		w.WriteBulk(pair.Value)
	}

	if err := w.Flush(); err != nil {
		c.setError(ret, addr, i, err)
		log.Warn("Failed to initiate setting %d@%s(%s): %v", i, key, addr, err)
		return
	}

	cn.conn.SetWriteDeadline(time.Time{})

	log.Debug("Initiated setting %d@%s(%s)", i, key, addr)
	c.mkRec("Set", addr, i, reqId, ret, nil)
}

func (c *Client) mkGet(addr string, key string, i int, lowLevelKeys set.Interface, reqId string, wg *sync.WaitGroup, ret *ecRet) {
	defer wg.Done()

	if err := c.validate(addr, i); err != nil {
		c.setError(ret, addr, i, err)
		log.Warn("Failed to validate connection %d@%s(%s): %v", i, key, addr, err)
		return
	}
	cn := c.Conns[addr][i]
	cn.conn.SetWriteDeadline(time.Now().Add(Timeout)) // Set deadline for request
	defer cn.conn.SetWriteDeadline(time.Time{})

	//tGet := time.Now()
	//fmt.Println("Client send GET req timeStamp", tGet, "chunkId is", i)

	w := cn.W
	w.WriteMultiBulkSize(7+lowLevelKeys.Size())
	w.WriteBulkString("mkget")
	w.WriteBulkString(key)
	w.WriteBulkString(strconv.Itoa(i))
	w.WriteBulkString(reqId)
	w.WriteBulkString(strconv.Itoa(c.MKReplicationFactors[0]))
	w.WriteBulkString(strconv.Itoa(0))
	w.WriteBulkString(strconv.Itoa(lowLevelKeys.Size()))
	lowLevelKeysList := lowLevelKeys.List()
	for i := 0; i < len(lowLevelKeysList); i++ {
		w.WriteBulkString(lowLevelKeysList[i].(string))
	}

	// Flush pipeline
	//if err := c.W[i].Flush(); err != nil {
	if err := cn.W.Flush(); err != nil {
		c.setError(ret, addr, i, err)
		log.Warn("Failed to initiate getting %d@%s(%s): %v", i, key, addr, err)
		return
	}
	cn.conn.SetWriteDeadline(time.Time{})

	log.Debug("Initiated getting %d@%s(%s)", i, key, addr)
	c.mkRec("Got", addr, i, reqId, ret, nil)
}

func (c *Client) mkRec(prompt string, addr string, i int, reqId string, ret *ecRet, wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}

	cn := c.Conns[addr][i]
	cn.conn.SetReadDeadline(time.Now().Add(Timeout)) // Set deadline for response
	defer cn.conn.SetReadDeadline(time.Time{})

	// peeking response type and receive
	// chunk id
	type0, err := cn.R.PeekType()
	if err != nil {
		log.Warn("PeekType error on receiving chunk %d: %v", i, err)
		c.setError(ret, addr, i, err)
		return
	}

	switch type0 {
	case resp.TypeError:
		strErr, err := c.Conns[addr][i].R.ReadError()
		if err == nil {
			err = errors.New(strErr)
		}
		log.Warn("Error on receiving chunk %d: %v", i, err)
		c.setError(ret, addr, i, err)
		return
	}

	respId, err := c.Conns[addr][i].R.ReadBulkString()
	if err != nil {
		log.Warn("Failed to read reqId on receiving chunk %d: %v", i, err)
		c.setError(ret, addr, i, err)
		return
	}
	if respId != reqId {
		log.Warn("Unexpected response %s, want %s, chunk %d", respId, reqId, i)
		// Skip fields
		_, _ = c.Conns[addr][i].R.ReadBulkString()
		_ = c.Conns[addr][i].R.SkipBulk()
		ret.SetError(i, ErrUnexpectedResponse)
		return
	}

	chunkId, err := c.Conns[addr][i].R.ReadBulkString()
	if err != nil {
		log.Warn("Failed to read chunkId on receiving chunk %d: %v", i, err)
		c.setError(ret, addr, i, err)
		return
	}
	if chunkId == "-1" {
		log.Debug("Abandon late chunk %d", i)
		return
	}

	// Read value
	valReader, err := c.Conns[addr][i].R.StreamBulk()
	if err != nil {
		log.Warn("Error on get value reader on receiving chunk %d: %v", i, err)
		c.setError(ret, addr, i, err)
		return
	}
	val, err := valReader.ReadAll()
	if err != nil {
		log.Error("Error on get value on receiving chunk %d: %v", i, err)
		c.setError(ret, addr, i, err)
		return
	}

	log.Debug("%s chunk %d", prompt, i)
	ret.Set(i, val)
}

func (c *Client) mkRecover(addr string, key string, reqId string, shards [][]byte, failed []int) {
	var wg sync.WaitGroup
	ret := newEcRet(c.Shards)
	for _, i := range failed {
		wg.Add(1)
		// lambdaId = 0, for lambdaID of a specified key is fixed on setting.
		go c.set(addr, key, shards[i], i, 0, reqId, &wg, ret)
	}
	wg.Wait()

	if ret.Err != nil {
		log.Warn("Failed to recover shards of %s: %v", key, failed)
	} else {
		log.Info("Succeeded to recover shards of %s: %v", key, failed)
	}
}

func replicate(groups [3]KVSetGroup, n int) []KVSetGroup {
	var replicas = make([]KVSetGroup, n)
	var rFs = []int{5,4,3}
	for i := 0; i < len(groups); i++ {
		var group = groups[i]
		for k := 0; k < len(group.KeyValuePairs); k++{
			for j := 0; j < rFs[i]; j++ {
				replicas[j].KeyValuePairs = append(replicas[j].KeyValuePairs, group.KeyValuePairs[k])
			}
		}
	}
	return replicas
}

func locateLowLevelKeys(groups [3]KVGetGroup) map[int]set.Interface {
	var replicas [3]int
	var m map[int]set.Interface

	fmt.Println(groups)

	for i:=0; i<len(groups); i++{
		g := groups[i]
		if g.Keys != nil {
			replicas[i] = rand.Intn(len(groups)+i)
		}else{
			replicas[i] = -1
		}
	}
	m = make(map[int]set.Interface)
	for i:=0; i<len(replicas); i++ {
		if replicas[i] >= 0 {
			if m[replicas[i]] == nil {
				m[replicas[i]] = set.New(set.ThreadSafe)
			}
			for k:=0; k<len(groups[i].Keys); k++ {
				m[replicas[i]].Add(groups[i].Keys[k])
			}
		}
	}

	return m
}