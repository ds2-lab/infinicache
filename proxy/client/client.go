package client

import "C"
import (
	"github.com/montanaflynn/stats"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/buraksezer/consistent"
	"github.com/klauspost/reedsolomon"
	"github.com/mason-leap-lab/redeo/resp"
	cuckoo "github.com/seiflotfy/cuckoofilter"
)

type Conn struct {
	conn net.Conn
	W    *resp.RequestWriter
	R    resp.ResponseReader
}

func (c *Conn) Close() {
	c.conn.Close()
}

type DataEntry struct {
	Cmd        string
	ReqId      string
	Begin      time.Time
	ReqLatency time.Duration
	RecLatency time.Duration
	Duration   time.Duration
	AllGood    bool
	Corrupted  bool
}

type Client struct {
	//ConnArr  []net.Conn
	//W        []*resp.RequestWriter
	//R        []resp.ResponseReader
	Conns        map[string][]*Conn
	EC           reedsolomon.Encoder
	MappingTable map[string]*cuckoo.Filter
	Ring         *consistent.Consistent
	Data         DataEntry
	DataShards   int
	ParityShards int
	Shards       int
	ReplicationFactor int
	MKReplicationFactors []int
	J int
	I int
}

func NewClient(dataShards int, parityShards int, ecMaxGoroutine int, replicationFactor int) *Client {
	return &Client{
		//ConnArr:  make([]net.Conn, dataShards+parityShards),
		//W:        make([]*resp.RequestWriter, dataShards+parityShards),
		//R:        make([]resp.ResponseReader, dataShards+parityShards),
		Conns:        make(map[string][]*Conn),
		EC:           NewEncoder(dataShards, parityShards, ecMaxGoroutine),
		MappingTable: make(map[string]*cuckoo.Filter),
		DataShards:   dataShards,
		ParityShards: parityShards,
		Shards:       dataShards + parityShards,
		ReplicationFactor: replicationFactor,
		MKReplicationFactors: []int{5,4,3},
		J: 1,
		I: 1,
	}
}

func (c *Client) Dial(addrArr []string) bool {
	//t0 := time.Now()
	members := []consistent.Member{}
	for _, host := range addrArr {
		member := Member(host)
		members = append(members, member)
	}
	//cfg := consistent.Config{
	//	PartitionCount:    271,
	//	ReplicationFactor: 20,
	//	Load:              1.25,
	//	Hasher:            hasher{},
	//}
	cfg := consistent.Config{
		PartitionCount:    271,
		ReplicationFactor: 20,
		Load:              1.25,
		Hasher:            hasher{},
	}
	c.Ring = consistent.New(members, cfg)
	for _, addr := range addrArr {
		log.Debug("Dialing %s...", addr)
		if err := c.initDial(addr); err != nil {
			log.Error("Fail to dial %s: %v", addr, err)
			c.Close()
			return false
		}
	}
	//time0 := time.Since(t0)
	//fmt.Println("Dial all goroutines are done!")
	//if err := nanolog.Log(LogClient, "Dial", time0.String()); err != nil {
	//	fmt.Println(err)
	//}
	return true
}

//func (c *Client) initDial(address string, wg *sync.WaitGroup) {
func (c *Client) initDial(address string) (err error) {
	// initialize parallel connections under address
	tmp := make([]*Conn, c.Shards)
	c.Conns[address] = tmp
	var i int
	for i = 0; i < c.Shards; i++ {
		err = c.connect(address, i)
		if err != nil {
			break
		}
	}
	if err == nil {
		// initialize the cuckoo filter under address
		c.MappingTable[address] = cuckoo.NewFilter(1000000)
	}

	return
}

func (c *Client) connect(address string, i int) error {
	cn, err := net.Dial("tcp", address)
	if err != nil {
		return err
	}
	c.Conns[address][i] = &Conn{
		conn: cn,
		W:    NewRequestWriter(cn),
		R:    NewResponseReader(cn),
	}
	return nil
}

func (c *Client) disconnect(address string, i int) {
	if c.Conns[address][i] != nil {
		c.Conns[address][i].Close()
		c.Conns[address][i] = nil
	}
}

func (c *Client) validate(address string, i int) error {
	if c.Conns[address][i] == nil {
		return c.connect(address, i)
	}

	return nil
}

func (c *Client) Close() {
	log.Info("Cleaning up...")
	for addr, conns := range c.Conns {
		for i, _ := range conns {
			c.disconnect(addr, i)
		}
	}
	log.Info("Client closed.")
}

func (c *Client) GenerateSetData(size int) [3]KVSetGroup{
	var data [3]KVSetGroup
	var g KVSetGroup
	val := make([]byte, size)
	rand.Read(val)
	c.J = c.I
	c.I = c.I +9
	counter := 0
	for ; c.J <= c.I; c.J++ {
		pair := KeyValuePair{Key: "k"+strconv.Itoa(c.J), Value: val}
		g.KeyValuePairs = append(g.KeyValuePairs, pair)
		if c.J%3 == 0 && c.J != 0 {
			data[counter] = g
			var newG KVSetGroup
			g = newG
			counter ++
		}
	}
	return data
}

func (c *Client) GenerateSetData2(size int, groups int) []KVSetGroup{
	var data []KVSetGroup
	var g KVSetGroup
	val := make([]byte, size)
	rand.Read(val)

	for x:=0 ; x < groups; x++ {
		pair := KeyValuePair{Key: "k"+strconv.Itoa(c.J), Value: val}
		g.KeyValuePairs = append(g.KeyValuePairs, pair)
		data = append(data, g)
		var newG KVSetGroup
		g = newG
		c.J++
	}
	return data
}

func (c *Client) RemoveBytes(data [3]KVSetGroup) [3]KVSetGroup{
	for i:=0;i<3;i++{
		g := data[i]
		for j:=0;j<len(g.KeyValuePairs);j++{
			pair := g.KeyValuePairs[j]
			pair.Value = nil
		}
	}
	return data
}

func (c *Client) GetStats(xs[]float64) (float64,float64,float64,float64,[]float64){
	mean, _ := stats.Mean(xs)
	percentile75, _ := stats.Percentile(xs, 75)
	percentile90, _ := stats.Percentile(xs, 90)
	percentile95, _ := stats.Percentile(xs, 95)
	percentile99, _ := stats.Percentile(xs, 99)
	percentiles := []float64{percentile75, percentile90, percentile95, percentile99}
	min, _ := stats.Min(xs)
	max, _ := stats.Max(xs)
	sd, _ := stats.StandardDeviation(xs)
	return min, max, mean, sd, percentiles
}

func GetArgs(args []string) (int,int,string){
	var proxies string
	proxiesOpt, err := strconv.Atoi(args[1])
	if err!=nil{
		log.Info("No arguments for test. number of proxies expected")
		os.Exit(1)
	}
	if proxiesOpt == 2 {
		proxies = "10.4.0.100:6378,10.4.14.71:6378"
	}else if proxiesOpt == 1{
		proxies = "10.4.0.100:6378"
	}else{
		log.Info("Unknown no of proxies on launch args")
		os.Exit(1)
	}
	requestsNumber, err := strconv.Atoi(args[2])
	if err!=nil{
		log.Info("No arguments for test. requests number expected")
		os.Exit(1)
	}
	size, err := strconv.Atoi(args[3])
	if err!=nil{
		log.Info("No arguments for test. size expected")
		os.Exit(1)
	}
	return requestsNumber,size,proxies
}

func GetArgsConcurrent(args []string) (int,int,int,string){
	var proxies string
	proxiesOpt, err := strconv.Atoi(args[1])
	if err!=nil{
		log.Info("No arguments for test. number of proxies expected")
		os.Exit(1)
	}
	if proxiesOpt == 2 {
		proxies = "10.4.0.100:6378,10.4.14.71:6378"
	}else if proxiesOpt == 1{
		proxies = "10.4.0.100:6378"
	}else{
		log.Info("Unknown no of proxies on launch args")
		os.Exit(1)
	}
	requestsNumber, err := strconv.Atoi(args[2])
	if err!=nil{
		log.Info("No arguments for test. requests number expected")
		os.Exit(1)
	}
	size, err := strconv.Atoi(args[3])
	if err!=nil{
		log.Info("No arguments for test. size expected")
		os.Exit(1)
	}
	clients, err := strconv.Atoi(args[4])
	if err!=nil{
		log.Info("No arguments for clients number. client number expected expected")
		os.Exit(1)
	}
	return requestsNumber,size,clients,proxies
}

func (c *Client) Average(xs[]float64)float64 {
	total:=float64(0)
	for _,v:=range xs {
		total +=v
	}
	return total/float64(len(xs))
}

func (c *Client) GenerateRandomGet(data [][]KVSetGroup) [][]KVGetGroup{
	var output [][]KVGetGroup

	for i:=0; i<len(data); i++{
		d := data[i]
		var getGroups []KVGetGroup = c.GenerateSingleRandomGet(d)
		output = append(output, getGroups)

	}
	return output
}

func (c *Client) GenerateSingleRandomGet(d []KVSetGroup) []KVGetGroup{

		var getGroups []KVGetGroup

		for j:=0; j<len(d); j++{
			g := d[j]
			var getG KVGetGroup
			var query []string
			query = append(query, g.KeyValuePairs[rand.Intn(len(g.KeyValuePairs))].Key)
			getG.Keys = query
			getGroups= append(getGroups, getG)
		}

		return getGroups
}

type ecRet struct {
	Shards int
	Rets   []interface{}
	Err    error
}

func newEcRet(shards int) *ecRet {
	return &ecRet{
		Shards: shards,
		Rets:   make([]interface{}, shards),
	}
}

func (r *ecRet) Len() int {
	return r.Shards
}

func (r *ecRet) Set(i int, ret interface{}) {
	r.Rets[i] = ret
}

func (r *ecRet) SetError(i int, ret interface{}) {
	r.Rets[i] = ret
	r.Err = ret.(error)
}

func (r *ecRet) Ret(i int) (ret []byte) {
	ret, _ = r.Rets[i].([]byte)
	return
}

func (r *ecRet) RetKVP(i int) (ret []KeyValuePair) {
	ret, _ = r.Rets[i].([]KeyValuePair)
	return
}

func (r *ecRet) Error(i int) (err error) {
	err, _ = r.Rets[i].(error)
	return
}
