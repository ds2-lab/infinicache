package types

import (
)

type ClientReqCounter struct {
	Cmd          string
	DataShards   int
	ParityShards int
	Counter      int32
}

type Id struct {
	ConnId  int
	ReqId   string
	ChunkId string
}

type Group struct {
	All        []LambdaInstance
	MemCounter uint64
}

type LambdaInstance interface {
	C() chan *Request
}
