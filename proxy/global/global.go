package global

import (
	"github.com/cornelk/hashmap"
	"github.com/mason-leap-lab/infinicache/common/logger"
	"sync"

	"github.com/mason-leap-lab/infinicache/proxy/types"
)

var (
	// Clients        = make([]chan interface{}, 1024*1024)
	DataCollected    sync.WaitGroup
	Log              logger.ILogger
	ReqMap           = hashmap.New(1024)
	Migrator         types.MigrationScheduler
	BasePort         = 6378
	BaseMigratorPort = 6380
	ServerIp         string
	Prefix           string
)

func init() {
	Log = logger.NilLogger

	ip, err := GetPrivateIp()
	if err != nil {
		panic(err)
	}

	ServerIp = ip
}
