package global

import (
	"github.com/cornelk/hashmap"
	"github.com/neboduus/infinicache/proxy/common/logger"
	"sync"

	"github.com/neboduus/infinicache/proxy/proxy/types"
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
	if ServerIp == "" {
		ip, err := GetPrivateIp()
		if err != nil {
			panic(err)
		}

		ServerIp = ip
	}
}
