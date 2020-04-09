package migrator

import (
	"errors"
	"fmt"
	"github.com/mason-leap-lab/redeo/resp"
	"strings"
	"sync"

	"github.com/neboduus/infinicache/node/lambda/types"
)

var (
	cmds = sync.Pool{
		New: func() interface{} {
			return &storageAdapterCommand{
				err: make(chan error),
			}
		},
	}
	ErrSkip = errors.New("Skiped")
)

type storageAdapterCommand struct {
	key        string
	chunk      string
	body       []byte
	bodyStream resp.AllReadCloser
	handler    func(*storageAdapterCommand)
	err        chan error
}

type StorageAdapter struct {
	migrator   *Client
	store      types.Storage
	serializer chan *storageAdapterCommand
	lastError  error
	done       chan struct{}
}

// Storage implementation
func newStorageAdapter(migrator *Client, store types.Storage) *StorageAdapter {
	adapter := &StorageAdapter{
		migrator:   migrator,
		store:      store,
		serializer: make(chan *storageAdapterCommand, 1),
		done:       make(chan struct{}),
	}

	go func() {
		for {
			select {
			case <-adapter.done:
				return
			case cmd := <-adapter.serializer:
				cmd.handler(cmd)
			}
		}
	}()

	return adapter
}

func (a *StorageAdapter) Restore() types.Storage {
	select {
	case <-a.done:
	default:
		close(a.done)
	}
	return a.store
}

func (a *StorageAdapter) Get(key string) (string, []byte, error) {
	chunkId, valReader, err := a.GetStream(key)
	if err != nil {
		return chunkId, nil, err
	}

	val, err := valReader.ReadAll()
	return chunkId, val, err
}

func (a *StorageAdapter) GetStream(key string) (string, resp.AllReadCloser, error) {
	cmd := cmds.Get().(*storageAdapterCommand)
	defer cmds.Put(cmd)

	cmd.key = key
	cmd.handler = a.getHandler
	a.serializer <- cmd

	err := <-cmd.err
	if err != nil {
		log.Warn("Proxying key %s: %v", cmd.key, err)
		return "", nil, err
	} else {
		return cmd.chunk, cmd.bodyStream, err
	}
}

func (a *StorageAdapter) Set(key string, chunk string, val []byte) error {
	return a.SetStream(key, chunk, resp.NewInlineReader(val))
}

func (a *StorageAdapter) SetStream(key string, chunk string, valReader resp.AllReadCloser) error {
	cmd := cmds.Get().(*storageAdapterCommand)
	defer cmds.Put(cmd)

	cmd.key = key
	cmd.chunk = chunk
	cmd.bodyStream = valReader
	cmd.handler = a.setHandler
	a.serializer <- cmd

	return <-cmd.err
}

func (a *StorageAdapter) Migrate(key string) (string, error) {
	cmd := cmds.Get().(*storageAdapterCommand)
	defer cmds.Put(cmd)

	cmd.key = key
	cmd.handler = a.migrateHandler
	a.serializer <- cmd

	err := <-cmd.err
	if err != nil {
		return "", err
	} else {
		return cmd.chunk, err
	}
}

func (a *StorageAdapter) Del(key string, chunk string) error {
	cmd := cmds.Get().(*storageAdapterCommand)
	defer cmds.Put(cmd)

	cmd.key = key
	cmd.chunk = chunk
	cmd.handler = a.delHandler
	a.serializer <- cmd

	return <-cmd.err
}

func (a *StorageAdapter) LocalDel(key string) {
	a.store.Del(key, "")
}
func (a *StorageAdapter) Len() int {
	return a.store.Len()
}

func (a *StorageAdapter) Keys() <-chan string {
	return a.store.Keys()
}

func (a *StorageAdapter) getHandler(cmd *storageAdapterCommand) {
	var err error
	cmd.chunk, cmd.bodyStream, err = a.store.GetStream(cmd.key)
	if err == nil {
		cmd.err <- nil
		return
	}

	reader, err := a.migrator.Send("get", nil, "migrator", "proxy", "", cmd.key)
	if err != nil {
		cmd.err <- err
		return
	}

	// Wait and read response
	err = a.readGetResponse(reader, cmd)
	if err != nil {
		cmd.err <- err
		return
	}

	// Intercept stream
	interceptor := NewInterceptReader(cmd.bodyStream)
	interceptor.AllReadCloser.(resp.Holdable).Hold() // Enable wait on closing.
	cmd.bodyStream = interceptor

	// return
	cmd.err <- nil

	// Wait until done streaming.
	interceptor.Close()

	// Hold released, check if any error exists
	if err := interceptor.LastError(); err != nil {
		log.Warn("Proxying key %s: %v", cmd.key, err)
		return
	}

	log.Debug("Proxying key %s(chunk %s): success", cmd.key, cmd.chunk)
	a.store.Set(cmd.key, cmd.chunk, interceptor.Intercepted())
}

func (a *StorageAdapter) setHandler(cmd *storageAdapterCommand) {
	// Intercept stream
	interceptor := NewInterceptReader(cmd.bodyStream)
	interceptor.AllReadCloser.(resp.Holdable).Hold() // Enable wait on closing.
	cmd.bodyStream = interceptor

	reader, err := a.migrator.Send("set", cmd.bodyStream, "migrator", "proxy", cmd.chunk, cmd.key)
	if err != nil {
		cmd.err <- err
		return
	}

	// Wait and read response
	err = a.readGetResponse(reader, cmd)
	if err != nil {
		cmd.err <- err
		return
	}

	// Streaming should done here, wait just in case.
	interceptor.Close()

	// Hold released, check if any error exists
	if err := interceptor.LastError(); err != nil {
		log.Warn("Unexpected error on forward setting key %s: %v", cmd.key, err)
		cmd.err <- err
		return
	}

	log.Debug("Forwarding key %s(chunk %s): success", cmd.key, cmd.chunk)
	a.store.Set(cmd.key, cmd.chunk, interceptor.Intercepted())
	// a.store.Set(cmd.key, cmd.chunk, cmd.body)
	cmd.err <- nil
}

func (a *StorageAdapter) migrateHandler(cmd *storageAdapterCommand) {
	var err error
	cmd.chunk, cmd.bodyStream, err = a.store.GetStream(cmd.key)
	if err == nil {
		cmd.err <- ErrSkip
		return
	}

	reader, err := a.migrator.Send("get", nil, "migrator", "migrate", "", cmd.key)
	if err != nil {
		cmd.err <- err
		return
	}

	// Wait and read response
	err = a.readGetResponse(reader, cmd)
	if err != nil {
		cmd.err <- err
		return
	}

	// Read stream
	body, err := cmd.bodyStream.ReadAll()
	if err != nil {
		cmd.err <- err
		return
	}

	a.store.Set(cmd.key, cmd.chunk, body)
	cmd.err <- nil
}

func (a *StorageAdapter) delHandler(cmd *storageAdapterCommand) {
	reader, err := a.migrator.Send("del", nil, "migrator", "proxy", cmd.chunk, cmd.key)
	if err != nil {
		cmd.err <- err
		return
	}

	// Wait and read response
	err = a.readGetResponse(reader, cmd)
	if err != nil {
		cmd.err <- err
		return
	}

	log.Debug("Forwarding Del cmd on key %s(chunk %s): success", cmd.key, cmd.chunk)
	a.store.Del(cmd.key, cmd.chunk)
	cmd.err <- nil
}

func (a *StorageAdapter) readGetResponse(reader resp.ResponseReader, cmd *storageAdapterCommand) (err error) {
	respType, err := reader.PeekType()
	if err != nil {
		a.lastError = err
		return ErrClosed
	}

	switch respType {
	case resp.TypeError:
		var strErr string
		strErr, err = reader.ReadError()
		if err == nil {
			err = errors.New(fmt.Sprintf("Error in migration response: %s", strErr))
		}
		return err
	}

	// cmd
	var cmdName string
	cmdName, err = reader.ReadBulkString()
	if err != nil {
		return err
	}
	// connId
	_, err = reader.ReadBulkString()
	if err != nil {
		return err
	}
	// reqId
	_, err = reader.ReadBulkString()
	if err != nil {
		return err
	}
	cmd.chunk, err = reader.ReadBulkString()
	if err != nil {
		return err
	}

	if strings.ToLower(cmdName) == "get" {
		cmd.bodyStream, err = reader.StreamBulk()
	}
	return nil
}
