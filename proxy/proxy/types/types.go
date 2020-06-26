package types

import (
	"errors"
)

var ErrNoSpareDeployment = errors.New("No spare deployment")

type ClientReqCounter struct {
	Cmd           string
	DataShards    int
	ParityShards  int
	Counter       int32
	LowLevelKeysN int
}

type Id struct {
	ConnId  int
	ReqId   string
	ChunkId string
}

type LambdaDeployment interface {
	Name() string
	Id() uint64
	Address() string
	Reset(new LambdaDeployment, old LambdaDeployment)
}

type MigrationScheduler interface {
	StartMigrator(uint64) (string, error)
	GetDestination(uint64) (LambdaDeployment, error)
}
