package client

import (
	"errors"
	"github.com/ScottMansfield/nanolog"
	"github.com/fatih/set"
	"github.com/google/uuid"
	"github.com/mason-leap-lab/redeo/resp"
	"math/rand"
	"strconv"
	"strings"
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

func (c *Client) MkSet(highLevelKey string, data []KVSetGroup, args ...interface{}) (string, float64, bool) {
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
		return stats.ReqId, 0, true
	}

	//addr, ok := c.getHost(highLevelKey)
	//fmt.Println("in SET, highLevelKey is: ", highLevelKey)
	member := c.Ring.LocateKey([]byte(highLevelKey))
	host := member.String()
	log.Debug("ring LocateKey costs: %v", time.Since(stats.Begin))
	log.Debug("SET located host: %s", host)

	var wg sync.WaitGroup
	ret := newEcRet(c.MKReplicationFactors[0])
	var replicas = c.replicate(data)

	for i := 0; i < len(replicas); i++ {
		// fmt.Println("shards", i, "is", shards[i])
		wg.Add(1)
		go c.mkSet(host, highLevelKey, replicas[i], i, index[i], stats.ReqId, &wg, ret)
	}
	wg.Wait()
	stats.ReqLatency = time.Since(stats.Begin)
	stats.Duration = stats.ReqLatency

	if ret.Err != nil {
		return stats.ReqId, 0, false
	}

	nanolog.Log(LogClient, "set", stats.ReqId, stats.Begin.UnixNano(),
		int64(stats.Duration), int64(stats.ReqLatency), int64(0), int64(0),
		false, false)
	//fmt.Println("mkset ", stats.ReqId, stats.Begin.UnixNano(),
	//	stats.Duration, float32(stats.Duration), int64(0), int64(stats.RecLatency),
	//	stats.AllGood, stats.Corrupted)

	if placements != nil {
		for i, ret := range ret.Rets {
			placements[i], _ = strconv.Atoi(string(ret.([]byte)))
		}
	}

	return stats.ReqId, stats.Duration.Seconds() * 1e3, true
}

func (c *Client) MkGet(highLevelKey string, lowLevelKeys []KVGetGroup) ([]KeyValuePair, float64, bool) {
	stats := &c.Data
	stats.Begin = time.Now()
	stats.ReqId = uuid.New().String()

	member := c.Ring.LocateKey([]byte(highLevelKey))
	host := member.String()

	// Send request and wait
	var wg sync.WaitGroup
	locations := c.LocateLowLevelKeys(lowLevelKeys)
	//fmt.Println(locations)
	ret := newEcRet(len(locations))
	j := 0
	for i, v := range locations {
		wg.Add(1)
		go c.mkGet(host, highLevelKey, j, v, stats.ReqId, &wg, ret, i)
		j++
	}
	wg.Wait()
	stats.RecLatency = time.Since(stats.Begin)

	// Filter results
	chunks := make([][]KeyValuePair, ret.Len())
	failed := make([]int, 0, ret.Len())
	for i, _ := range ret.Rets {
		err := ret.Error(i)
		if err != nil {
			failed = append(failed, i)
		} else {
			chunks[i] = ret.RetKVP(i)
		}
	}

	keyValuePairs := c.Reasemble(chunks)
	// Try recover
	if len(failed) > 0 {
		// c.recover(host, highLevelKey, uuid.New().String(), chunks, failed)
		log.Error("Some of the chunks are faled, try another replica!")
		return nil, -1, false
	}

	end := time.Now()
	stats.Duration = end.Sub(stats.Begin)
	//fmt.Println("mkget", stats.ReqId, stats.Begin.UnixNano(),
	//	stats.Duration, int64(0), int64(stats.RecLatency),
	//	stats.AllGood, stats.Corrupted)

	return keyValuePairs, stats.Duration.Seconds() * 1e3, true
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
	c.mkRec("mkSet", addr, i, reqId, ret, nil, i)
}

func (c *Client) mkGet(addr string, key string, i int, lowLevelKeys set.Interface, reqId string, wg *sync.WaitGroup, ret *ecRet, j int) {
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
	w.WriteBulkString(strconv.Itoa(j))
	w.WriteBulkString(reqId)
	w.WriteBulkString(strconv.Itoa(c.MKReplicationFactors[0]))
	w.WriteBulkString(strconv.Itoa(0))
	w.WriteBulkString(strconv.Itoa(lowLevelKeys.Size()))
	lowLevelKeysList := lowLevelKeys.List()
	for m := 0; m < len(lowLevelKeysList); m++ {
		w.WriteBulkString(lowLevelKeysList[m].(string))
	}

	// Flush pipeline
	//if err := c.W[i].Flush(); err != nil {
	if err := cn.W.Flush(); err != nil {
		c.setError(ret, addr, i, err)
		log.Warn("Failed to initiate getting %d@%s(%s): %v", i, key, addr, err)
		return
	}
	cn.conn.SetWriteDeadline(time.Time{})

	c.mkRec("mkGot", addr, i, reqId, ret, nil, j)
}

func (c *Client) mkRec(prompt string, addr string, i int, reqId string, ret *ecRet, wg *sync.WaitGroup, j int) {
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

	if strings.Compare(prompt, "mkGot") == 0{
		lowLevelKeyValuePairsN, _ := c.Conns[addr][i].R.ReadInt()
		var keyValuePairs []KeyValuePair

		for m:=0; m<int(lowLevelKeyValuePairsN); m++ {
			lowLevelKey, _ := c.Conns[addr][i].R.ReadBulkString()
			var value []byte
			var err error
			value, err = c.Conns[addr][i].R.ReadBulk(value)
			if err != nil {
				log.Warn("Error on mkgeting value from chunk %d: %v", i, err)
				c.setError(ret, addr, i, err)
				return
			}
			pair := KeyValuePair{
				Key: lowLevelKey,
				Value: value,
			}
			keyValuePairs = append(keyValuePairs, pair)
		}
		ret.Set(i, keyValuePairs)
	}else{
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

		ret.Set(i, val)
	}

	log.Debug("%s chunk %d", prompt, i)
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

func (c *Client) replicate(groups []KVSetGroup) []KVSetGroup {
	_, maxRF := minMax(c.MKReplicationFactors)
	var replicas = make([]KVSetGroup, maxRF)
	var rFs = c.MKReplicationFactors

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

func minMax(array []int) (int, int) {
	var max int = array[0]
	var min int = array[0]
	for _, value := range array {
		if max < value {
			max = value
		}
		if min > value {
			min = value
		}
	}
	return min, max
}

func (c *Client) LocateLowLevelKeys(groups []KVGetGroup) map[int]set.Interface {
	replicas := make([]int, len(groups))

	var m map[int]set.Interface
	for i:=0; i<len(groups); i++{
		g := groups[i]
		if g.Keys != nil {
			replicas[i] = rand.Intn(5-i)
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
	//fmt.Println("Located: ", m)
	return m
}

func (c *Client) Reasemble(keyValuePairs [][]KeyValuePair) (assembled []KeyValuePair) {
	for _, kvps := range keyValuePairs {
		for _, kvp := range kvps {
			assembled = append(assembled, kvp)
		}
	}
	return
}