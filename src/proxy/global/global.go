package global

import (
	"github.com/cornelk/hashmap"
	"github.com/wangaoone/LambdaObjectstore/lib/logger"
	"sync"
)

var (
	// Clients        = make([]chan interface{}, 1024*1024)
	DataCollected   sync.WaitGroup
	Log             logger.ILogger
	ReqMap          = hashmap.New(1024)
)
