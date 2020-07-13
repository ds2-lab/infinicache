package collector

// This package has been implemented to work with S3
// ToDo: Integrate some other reliable store, maybe PostgreSQL, or Firebase

import (
	"bytes"
	"fmt"
	"github.com/neboduus/infinicache/node/common/logger"
	"os/exec"
	"strings"
	"sync"

	"github.com/neboduus/infinicache/node/lambda/lifetime"
	"github.com/neboduus/infinicache/node/lambda/types"
)


var (
	Prefix         string
	HostName       string
	FunctionName   string = "- UNSET -"

	dataGatherer                   = make(chan *types.DataEntry, 10)
	dataDepository                 = make([]*types.DataEntry, 0, 100)
	dataDeposited  sync.WaitGroup
	log            logger.ILogger  = &logger.ColorLogger{ Prefix: "collector ", Level: logger.LOG_LEVEL_INFO }
)

func init() {
	cmd := exec.Command("uname", "-a")
	host, err := cmd.CombinedOutput()
	if err != nil {
		log.Debug("cmd.Run() failed with %s\n", err)
	}

	HostName = strings.Split(string(host), " #")[0]
	log.Debug("hostname is: %s", HostName)

}

func Send(entry *types.DataEntry) {
	dataDeposited.Add(1)
	dataGatherer <- entry
}

func Collect(session *lifetime.Session) {
	session.Clear.Add(1)
	defer session.Clear.Done()

	for {
		select {
		case <-session.WaitDone():
			return
		case entry := <-dataGatherer:
			dataDepository = append(dataDepository, entry)
			dataDeposited.Done()
		}
	}
}

func Save(l *lifetime.Lifetime) {
	// Wait for data depository.
	dataDeposited.Wait()

	var data bytes.Buffer
	for _, entry := range dataDepository {
		data.WriteString(fmt.Sprintf("%d,%s,%s,%s,%d,%d,%d,%s,%s,%s\n",
			entry.Op, entry.ReqId, entry.ChunkId, entry.Status,
			entry.Duration, entry.DurationAppend, entry.DurationFlush,
			HostName, FunctionName, entry.Session))
	}

	key := fmt.Sprintf("%s/%s/%d", Prefix, FunctionName, l.Id())
	Put("someLocation", key, data.String())
	dataDepository = dataDepository[:0]
}

func Put(bucket string, key string, f string) {
	// Perform an upload.

	log.Info("Data should be uploaded to some reliable STORE (e.g. S3)")
}

func SetFunctionName(fName string){
	FunctionName = fName
}


