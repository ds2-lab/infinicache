package migrator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mason-leap-lab/redeo"
	"github.com/mason-leap-lab/redeo/resp"
	"github.com/neboduus/infinicache/node/common/logger"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/neboduus/infinicache/node/lambda/types"
)

const (
	AWSRegion                      = "us-east-1"
)

var (
	log = &logger.ColorLogger{
		Level: logger.LOG_LEVEL_INFO,
	}
	MigrationTimeout     = 30 * time.Second
	ErrClosedPrematurely = errors.New("Client closed before ready.")
	ErrClosed            = errors.New("Client closed.")
)

type Client struct {
	addr  string
	cn    net.Conn
	ready chan error
	mu    sync.Mutex
	w     *resp.RequestWriter
	r     resp.ResponseReader
}

func NewClient() *Client {
	return &Client{
		ready: make(chan error, 1), // We don't want to block the channel.
	}
}

func (cli *Client) Initiate(initiator func() error) error {
	err := initiator()
	if err != nil {
		return err
	}

	// Test ready and reset if neccessary
	select {
	case err := <-cli.ready:
		if err == nil {
			// closed, reopen
			cli.ready = make(chan error, 1)
		}
	default:
	}

	return <-cli.ready
}

func (cli *Client) Connect(addr string) (err error) {
	cli.cn, err = net.Dial("tcp", addr)
	if err != nil {
		cli.ready <- err
		return
	}

	// FIXME: Time out not working.
	// Fix attemption 20191221(To be confirm): Read timeout not return in StorageAdapter::readGetResponse.
	cli.cn.SetDeadline(time.Now().Add(MigrationTimeout))
	return
}

func (cli *Client) TriggerDestination(dest string, args interface{}) (err error) {
	p := new(bytes.Buffer)
	json.NewEncoder(p).Encode(args)
	res, err := http.Post(dest, "application/json", p)

	if err == nil && res.StatusCode >= 300 {
		err = errors.New(fmt.Sprintf("Unexpected http code on triggering destination of migration: %d", res.StatusCode))
	}
	if err != nil {
		cli.ready <- err
	}
	return
}

func (cli *Client) Send(cmd string, stream resp.AllReadCloser, args ...string) (resp.ResponseReader, error) {
	if cli.w == nil && cli.r == nil {
		cli.w = resp.NewRequestWriter(cli.cn)
		cli.r = resp.NewResponseReader(cli.cn)
	}

	if stream == nil {
		cli.w.WriteMultiBulkSize(len(args) + 1)
	} else {
		cli.w.WriteMultiBulkSize(len(args) + 2)
	}
	cli.w.WriteBulkString(cmd)
	for _, arg := range args {
		cli.w.WriteBulkString(arg)
	}
	if stream != nil {
		if err := cli.w.CopyBulk(stream, stream.Len()); err != nil {
			return nil, err
		}
	}
	if err := cli.w.Flush(); err != nil {
		return nil, err
	}

	return cli.r, nil
}

func (cli *Client) WaitForMigration(srv *redeo.Server) {
	defer cli.Close()

	err := srv.ServeForeignClient(cli.cn)
	if err == nil {
		return
	} else if !cli.IsReady() {
		if err == io.EOF {
			err = ErrClosedPrematurely
		}
		log.Warn("Migration connection closed: %v", err)
		cli.ready <- err
	} else if err != io.EOF {
		log.Warn("Migration connection closed: %v", err)
	} else {
		log.Info("Migration Connection closed.")
	}
}

func (cli *Client) Migrate(reader resp.ResponseReader, store types.Storage) {
	defer cli.Close()

	reader.ReadBulkString() // skip command
	strLen, err := reader.ReadBulkString()
	len := 0
	if err != nil {
		log.Error("Failed to read length of data from migrator: %v", err)
		return
	} else {
		len, err = strconv.Atoi(strLen)
		if err != nil {
			log.Error("Convert strLen err: %v", err)
			return
		}
	}

	keys := make([]string, len)
	for i := 0; i < len; i++ {
		keys[i], err = reader.ReadBulkString()
		if err != nil {
			log.Error("Failed to read migration keys: %v", err)
			return
		}
	}

	// only determine del or get for now.
	opDel := strconv.Itoa(types.OP_DEL)

	// Start migration
	log.Info("Start migrating %d keys", len)
	for _, cmdkey := range keys {
		key := string(cmdkey[1:])
		if opDel == string(cmdkey[0]) {
			store.(*StorageAdapter).LocalDel(key)
			log.Debug("Flaging %s deleted", key)
			continue
		}

		chunk, err := store.(*StorageAdapter).Migrate(key)
		if err == ErrSkip {
			log.Debug("Migrating key %s: %v", key, err)
		} else if err == ErrClosed {
			log.Warn("Migration connection closed unexpectedly: %v", store.(*StorageAdapter).lastError)
			return
		} else if err != nil {
			log.Warn("Migrating key %s: %v", key, err)
		} else {
			log.Debug("Migrating key %s(chunk %s): success", key, chunk)
		}
	}
	log.Info("End migration")
}

func (cli *Client) SetError(err error) {
	select {
	case err := <-cli.ready:
		if err == nil {
			// Client is ready and closed
			return
		}
		// Or overwrite with new err
	default:
	}

	cli.ready <- err
}

func (cli *Client) SetReady() {
	select {
	case <-cli.ready:
		return
	default:
	}

	close(cli.ready)
}

func (cli *Client) Ready() <-chan error {
	return cli.ready
}

func (cli *Client) IsReady() bool {
	select {
	case err := <-cli.ready:
		if err == nil {
			return true
		} else {
			cli.ready <- err
			return false
		}
	default:
		return false
	}
}

func (cli *Client) GetStoreAdapter(store types.Storage) *StorageAdapter {
	return newStorageAdapter(cli, store)
}

func (cli *Client) Close() {
	cli.cn.Close()
}
