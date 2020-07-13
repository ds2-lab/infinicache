package client

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/ScottMansfield/nanolog"
	"github.com/cespare/xxhash"
	"github.com/google/uuid"
	"github.com/mason-leap-lab/redeo/resp"
	"github.com/neboduus/infinicache/proxy/common/logger"
	"github.com/neboduus/infinicache/proxy/proxy/server"
)

const (
	// This setting will avoid network contention.
	MaxLambdaStores int = server.NumLambdaClusters
	Timeout             = 2 * time.Minute
)

var (
	log = &logger.ColorLogger{
		Prefix: "EcRedis ",
		Level:  logger.LOG_LEVEL_INFO,
		Color:  true,
	}
	ErrUnexpectedResponse = errors.New("Unexpected response")
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type Member string

func (m Member) String() string {
	return string(m)
}

type hasher struct{}

func (h hasher) Sum64(data []byte) uint64 {
	return xxhash.Sum64(data)
}

func NewRequestWriter(wr io.Writer) *resp.RequestWriter {
	return resp.NewRequestWriter(wr)
}
func NewResponseReader(rd io.Reader) resp.ResponseReader {
	return resp.NewResponseReader(rd)
}

func (c *Client) EcSet(key string, val []byte, args ...interface{}) (string, float64, bool) {
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
	index := random(numClusters, c.Shards)
	if dryrun > 0 && placements != nil {
		for i, ret := range index {
			placements[i] = ret
		}
		return stats.ReqId, -1, true
	}

	//addr, ok := c.getHost(key)
	//fmt.Println("in SET, key is: ", key)
	member := c.Ring.LocateKey([]byte(key))
	host := member.String()
	log.Debug("ring LocateKey costs: %v", time.Since(stats.Begin))
	log.Debug("SET located host: %s", host)

	shards, err := c.encode(val)
	if err != nil {
		log.Warn("EcSet failed to encode: %v", err)
		return stats.ReqId, -1, false
	}

	var wg sync.WaitGroup
	ret := newEcRet(c.Shards)
	for i := 0; i < ret.Len(); i++ {
		// fmt.Println("shards", i, "is", shards[i])
		wg.Add(1)
		go c.set(host, key, shards[i], i, index[i], stats.ReqId, &wg, ret)
	}
	wg.Wait()
	stats.ReqLatency = time.Since(stats.Begin)
	stats.Duration = stats.ReqLatency

	if ret.Err != nil {
		return stats.ReqId, -1, false
	}

	nanolog.Log(LogClient, "set", stats.ReqId, stats.Begin.UnixNano(),
		int64(stats.Duration), int64(stats.ReqLatency), int64(0), int64(0),
		false, false)
	//log.Info("Set %s %d", key, stats.Duration.Seconds() * 1e3)

	if placements != nil {
		for i, ret := range ret.Rets {
			placements[i], _ = strconv.Atoi(string(ret.([]byte)))
		}
	}

	return stats.ReqId, stats.Duration.Seconds() * 1e3, true
}

func (c *Client) EcGet(key string, size int, args ...interface{}) (string, io.ReadCloser, float64, bool) {
	var dryrun int
	if len(args) > 0 {
		dryrun, _ = args[0].(int)
	}

	stats := &c.Data
	stats.Begin = time.Now()
	stats.ReqId = uuid.New().String()
	if dryrun > 0 {
		return stats.ReqId, nil, -1, true
	}

	//addr, ok := c.getHost(key)
	member := c.Ring.LocateKey([]byte(key))
	host := member.String()
	//fmt.Println("ring LocateKey costs:", time.Since(t))
	//fmt.Println("GET located host: ", host)

	// Send request and wait
	var wg sync.WaitGroup
	ret := newEcRet(c.Shards)
	for i := 0; i < ret.Len(); i++ {
		wg.Add(1)
		go c.get(host, key, i, stats.ReqId, &wg, ret)
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
			fmt.Println(err)
		} else {
			chunks[i] = ret.Ret(i)
		}
	}

	decodeStart := time.Now()
	reader, err := c.decode(stats, chunks, size)
	if err != nil {
		return stats.ReqId, nil, -1, false
	}

	end := time.Now()
	stats.Duration = end.Sub(stats.Begin)
	nanolog.Log(LogClient, "get", stats.ReqId, stats.Begin.UnixNano(),
		int64(stats.Duration), int64(0), int64(stats.RecLatency), int64(end.Sub(decodeStart)),
		stats.AllGood, stats.Corrupted)
	//fmt.Println("Got", key, stats.Duration, int64(stats.RecLatency), int64(end.Sub(decodeStart)))

	// Try recover
	if len(failed) > 0 {
		//fmt.Println("Recovering ", failed)
		c.recover(host, key, uuid.New().String(), chunks, failed)
		return stats.ReqId, reader, stats.Duration.Seconds() * 1e3, false
	}

	return stats.ReqId, reader, stats.Duration.Seconds() * 1e3, true
}

func (c *Client) getHost(key string) (addr string, ok bool) {
	// linear search through all filters and locate the one that holds the key
	for addr, filter := range c.MappingTable {
		found := filter.Lookup([]byte(key))
		if found { // if found, return the address
			return addr, true
		}
	}
	// otherwise, return nil
	return "", false
}

// random will generate random sequence within the lambda stores
// index and get top n id
func random(cluster, n int) []int {
	return rand.Perm(cluster)[:n]
}

func (c *Client) setError(ret *ecRet, addr string, i int, err error) {
	if err == io.EOF {
		c.disconnect(addr, i)
	} else if err, ok := err.(net.Error); ok && err.Timeout() {
		c.disconnect(addr, i)
	}
	ret.SetError(i, err)
}

func (c *Client) set(addr string, key string, val []byte, i int, lambdaId int, reqId string, wg *sync.WaitGroup, ret *ecRet) {
	defer wg.Done()

	if err := c.validate(addr, i); err != nil {
		c.setError(ret, addr, i, err)
		log.Warn("Failed to validate connection %d@%s(%s): %v", i, key, addr, err)
		return
	}
	cn := c.Conns[addr][i]
	cn.conn.SetWriteDeadline(time.Now().Add(Timeout)) // Set deadline for request
	defer cn.conn.SetWriteDeadline(time.Time{})

	w := cn.W
	w.WriteMultiBulkSize(9)
	w.WriteBulkString("set")
	w.WriteBulkString(key)
	w.WriteBulkString(strconv.Itoa(i))
	w.WriteBulkString(strconv.Itoa(lambdaId))
	w.WriteBulkString(strconv.Itoa(MaxLambdaStores))
	w.WriteBulkString(reqId)
	w.WriteBulkString(strconv.Itoa(c.DataShards))
	w.WriteBulkString(strconv.Itoa(c.ParityShards))

	// Flush pipeline
	//if err := c.W[i].Flush(); err != nil {
	if err := w.CopyBulk(bytes.NewReader(val), int64(len(val))); err != nil {
		c.setError(ret, addr, i, err)
		log.Warn("Failed to initiate setting %d@%s(%s): %v", i, key, addr, err)
		return
	}
	if err := w.Flush(); err != nil {
		c.setError(ret, addr, i, err)
		log.Warn("Failed to initiate setting %d@%s(%s): %v", i, key, addr, err)
		return
	}
	cn.conn.SetWriteDeadline(time.Time{})

	log.Debug("Initiated setting %d@%s(%s)", i, key, addr)
	c.rec("Set", addr, i, reqId, ret, nil)
}

func (c *Client) get(addr string, key string, i int, reqId string, wg *sync.WaitGroup, ret *ecRet) {
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
	cn.W.WriteCmdString(
		"get", key, strconv.Itoa(i),
		reqId, strconv.Itoa(c.DataShards), strconv.Itoa(c.ParityShards)) // cmd key chunkId reqId DataShards ParityShards

	// Flush pipeline
	//if err := c.W[i].Flush(); err != nil {
	if err := cn.W.Flush(); err != nil {
		c.setError(ret, addr, i, err)
		log.Warn("Failed to initiate getting %d@%s(%s): %v", i, key, addr, err)
		return
	}
	cn.conn.SetWriteDeadline(time.Time{})

	log.Debug("Initiated getting %d@%s(%s)", i, key, addr)
	c.rec("Got", addr, i, reqId, ret, nil)
}

func (c *Client) rec(prompt string, addr string, i int, reqId string, ret *ecRet, wg *sync.WaitGroup) {
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

func (c *Client) recover(addr string, key string, reqId string, shards [][]byte, failed []int) {
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

func (c *Client) encode(obj []byte) ([][]byte, error) {
	// split obj first
	shards, err := c.EC.Split(obj)
	if err != nil {
		log.Warn("Encoding split err: %v", err)
		return nil, err
	}
	// Encode parity
	err = c.EC.Encode(shards)
	if err != nil {
		log.Warn("Encoding encode err: %v", err)
		return nil, err
	}
	ok, err := c.EC.Verify(shards)
	if ok == false {
		log.Warn("Failed to verify encoding: %v", err)
		return nil, err
	}
	log.Debug("Encoding succeeded.")
	return shards, err
}

func (c *Client) decode(stats *DataEntry, data [][]byte, size int) (io.ReadCloser, error) {
	// var err error
	stats.AllGood, _ = c.EC.Verify(data)
	if stats.AllGood {
		log.Debug("No reconstruction needed.")
		// } else if err != nil {
		// 	stats.Corrupted = true
		// 	log.Debug("Verification error, impossible to reconstructing data: %v", err)
		// 	return nil, err
	} else {
		log.Debug("Verification failed. Reconstructing data...")
		err := c.EC.Reconstruct(data)
		if err != nil {
			log.Warn("Reconstruction failed: %v", err)
			return nil, err
		}
		stats.Corrupted, err = c.EC.Verify(data)
		if !stats.Corrupted {
			log.Warn("Verification failed after reconstruction, data could be corrupted: %v", err)
			return nil, err
		} else {
			log.Debug("Reconstructed")
		}
	}

	reader, writer := io.Pipe()
	go c.EC.Join(writer, data, size)
	return reader, nil
}