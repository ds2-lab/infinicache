package storage

import (
	"errors"
	"fmt"
	"github.com/neboduus/infinicache/node/lambda/types"
	"github.com/mason-leap-lab/redeo/resp"
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

func (s *Storage) Get(key string) (string, []byte, error) {
	chunk, ok := s.repo[key]
	if !ok || chunk.Body == nil {
		return "", nil, types.ErrNotFound
	}

	return chunk.Id, chunk.Access(), nil
}

func (s *Storage) Del(key string, chunkId string) error {
	chunk, ok := s.repo[key]
	if !ok {
		return types.ErrNotFound
	}
	chunk.Access()

	chunk.Body = nil
	return nil
}

func (s *Storage) GetStream(key string) (string, resp.AllReadCloser, error) {
	chunkId, val, err := s.Get(key)
	if err != nil {
		return chunkId, nil, err
	}

	return chunkId, resp.NewInlineReader(val), nil
}

func (s *Storage) Set(key string, chunkId string, val []byte) error {
	chunk := types.NewChunk(chunkId, val)
	chunk.Key = key
	s.repo[key] = chunk
	return nil
}

func (s *Storage) SetStream(key string, chunkId string, valReader resp.AllReadCloser) error {
	val, err := valReader.ReadAll()
	if err != nil {
		return errors.New(fmt.Sprintf("Error on read stream: %v", err))
	}

	return s.Set(key, chunkId, val)
}

func (s *Storage) Len() int {
	return len(s.repo)
}

func (s *Storage) Keys() <-chan string {
	// Gather and send key list. We expected num of keys to be small
	all := make([]*types.Chunk, 0, len(s.repo))
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
