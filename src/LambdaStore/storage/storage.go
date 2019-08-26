package storage

import (
	"github.com/wangaoone/LambdaObjectstore/src/LambdaStore/types"
	"github.com/wangaoone/redeo/resp"
	"sort"
)

type Storage struct {
	repo map[string]*types.Chunk
}

func New() *Storage {
	return &Storage{
		repo: make(map[string]*types.Chunk),
	}
}

func (s *Storage) Get(key string) (string, resp.AllReadCloser, error) {
	chunk, ok := s.repo[key]
	if !ok {
		return "", nil, types.ErrNotFound
	}

	return chunk.Id, resp.NewInlineReader(chunk.Access()), nil
}

func (s *Storage) Set(key string, chunkId string, body []byte) {
	chunk := types.NewChunk(chunkId, body)
	chunk.Key = key
	s.repo[key] = chunk
}

func (s *Storage) Len() int {
	return len(s.repo)
}

func (s *Storage) Keys() <-chan string {
	// Gather and send key list. We expected num of keys to be small
	all := make([]*types.Chunk, len(s.repo))
	for _, chunk := range s.repo {
		all = append(all, chunk)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].Accessed.UnixNano() > all[j].Accessed.UnixNano() 
	})

	ch := make(chan string)
	go func() {
		for _, chunk := range all {
			ch <- chunk.Key
		}
		close(ch)
	}()

	return ch
}
