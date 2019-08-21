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

type LambdaDeployment interface {
	Name() string
	Id() uint64
	Reset(new LambdaDeployment, old LambdaDeployment)
}
