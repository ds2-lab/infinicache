package migrator

import (
	"errors"
	"fmt"
	"github.com/wangaoone/redeo/resp"
	"sync"

	"github.com/wangaoone/LambdaObjectstore/src/LambdaStore/types"
)

var (
	cmds  = sync.Pool{
		New: func() interface{} {
			return &storageAdapterCommand{
				err: make(chan error),
			}
		},
	}
  ErrSkip = errors.New("Skiped")
)

type storageAdapterCommand struct {
	key    string
	chunk  string
	body   []byte
	bodyStream resp.AllReadCloser
	handler func(*storageAdapterCommand)
	err    chan error
}

type StorageAdapter struct {
	migrator     *Client
	store        types.Storage
	serializer   chan *storageAdapterCommand
	done         chan struct{}
}

// Storage implementation
func newStorageAdapter(migrator *Client, store types.Storage) *StorageAdapter {
	adapter := &StorageAdapter{
		migrator: migrator,
		store: store,
		serializer: make(chan *storageAdapterCommand, 1),
		done: make(chan struct{}),
	}

	go func() {
		for {
			select{
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

func (a *StorageAdapter) Get(key string) (string, resp.AllReadCloser, error) {
	cmd := cmds.Get().(*storageAdapterCommand)
	defer cmds.Put(cmd)

	cmd.key = key
	cmd.handler = a.getHandler
	a.serializer<- cmd

	err := <-cmd.err
	if err != nil {
		return "", nil, err
	} else {
		return cmd.chunk, cmd.bodyStream, err
	}
}

func (a *StorageAdapter) Set(key string, chunk string, body []byte) {
	cmd := cmds.Get().(*storageAdapterCommand)
	defer cmds.Put(cmd)

	cmd.key = key
	cmd.chunk = chunk
	cmd.body = body
	cmd.handler = a.setHandler
	a.serializer<- cmd

	<-cmd.err
}

func (a *StorageAdapter) Migrate(key string) (string, error) {
	cmd := cmds.Get().(*storageAdapterCommand)
	defer cmds.Put(cmd)

	cmd.key = key
	cmd.handler = a.migrateHandler
	a.serializer<- cmd

	err := <-cmd.err
	if err != nil {
		return "", err
	} else {
		return cmd.chunk, err
	}
}

func (a *StorageAdapter) Len() int {
	return a.store.Len()
}

func (a *StorageAdapter) Keys() <-chan string {
	return a.store.Keys()
}

func (a *StorageAdapter) getHandler(cmd *storageAdapterCommand) {
	var err error
	cmd.chunk, cmd.bodyStream, err = a.store.Get(cmd.key)
	if err == nil {
		cmd.err<- nil
		return
	}

	reader, err := a.migrator.Send("get", "migrator", "proxy", "", cmd.key)
	if err != nil {
		cmd.err<- err
		return
	}

	// Wait and read response
	err = a.readGetResponse(reader, cmd)
	if err != nil {
		cmd.err<- err
		return
	}

	// Intercept stream
	interceptor := NewInterceptReader(cmd.bodyStream)
	cmd.bodyStream = interceptor
	a.store.Set(cmd.key, cmd.chunk, interceptor.Intercepted())

	// return
	cmd.err<- nil

	// Hold until done streaming
	interceptor.AllReadCloser.(resp.Holdable).Hold()
	interceptor.Close()
}

func (a *StorageAdapter) setHandler(cmd *storageAdapterCommand) {
	a.store.Set(cmd.key, cmd.chunk, cmd.body)
	cmd.err<- nil
}

func (a *StorageAdapter) migrateHandler(cmd *storageAdapterCommand) {
	var err error
	cmd.chunk, cmd.bodyStream, err = a.store.Get(cmd.key)
	if err == nil {
		cmd.err<- ErrSkip
		return
	}

	reader, err := a.migrator.Send("get", "migrator", "migrate", "", cmd.key)
	if err != nil {
		cmd.err<- err
		return
	}

	// Wait and read response
	err = a.readGetResponse(reader, cmd)
	if err != nil {
		cmd.err<- err
		return
	}

	// Read stream
	body, err := cmd.bodyStream.ReadAll()
	if err != nil {
		cmd.err<- err
		return
	}

	a.store.Set(cmd.key, cmd.chunk, body)
	cmd.err<- nil
}

func (a *StorageAdapter) readGetResponse(reader resp.ResponseReader, cmd *storageAdapterCommand) (err error) {
	respType, err := reader.PeekType()
	if err != nil {
		return
	}

	switch respType {
	case resp.TypeError:
		var strErr string
		strErr, err = reader.ReadError()
		if err == nil {
			err = errors.New(fmt.Sprintf("Error in migration response: %s", strErr))
		}
		return
	}

	// cmd
	_, err = reader.ReadBulkString()
	if err != nil {
		return
	}
	// connId
	_, err = reader.ReadBulkString()
	if err != nil {
		return
	}
	// reqId
	_, err = reader.ReadBulkString()
	if err != nil {
		return
	}
	cmd.chunk, err = reader.ReadBulkString()
	if err != nil {
		return
	}
	cmd.bodyStream, err = reader.StreamBulk()
	return
}
