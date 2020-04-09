package server

import (
	"github.com/cornelk/hashmap"
)

type MetaPostProcess func(MetaDoPostProcess)

type MetaDoPostProcess func(*Meta)

type MetaStore struct {
	metaMap *hashmap.HashMap
}

func NewMataStore() *MetaStore {
	return &MetaStore{ metaMap: hashmap.New(1024) }
}

func NewMataStoreWithCapacity(size uintptr) *MetaStore {
	return &MetaStore{ metaMap: hashmap.New(size) }
}

func (ms *MetaStore) GetOrInsert(key string, insert *Meta) (*Meta, bool, MetaPostProcess) {
	meta, ok := ms.metaMap.GetOrInsert(key, insert)
	return meta.(*Meta), ok, nil
}

func (ms *MetaStore) Get(key string) (*Meta, bool) {
	meta, ok := ms.metaMap.Get(key)
	if ok {
		return meta.(*Meta), ok
	} else {
		return nil, ok
	}
}
