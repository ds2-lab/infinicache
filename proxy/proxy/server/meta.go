package server

import(
	"fmt"
	"sync"
)

var (
	metaPool = sync.Pool{
		New: func() interface{} {
				return &Meta{}
		},
	}
	emptyPlacement = make(Placement, 100)
)

type Meta struct {
	Key         string
	NumChunks   int
	Placement
	ChunkSize   int64
	Reset       bool
	Deleted     bool

	slice       Slice
	placerMeta  *PlacerMeta
	lastChunk   int
	mu          sync.Mutex
}

func NewMeta(key string, numChunks int, chunkSize int64) *Meta {
	meta := metaPool.Get().(*Meta)
	meta.Key = key
	meta.NumChunks = numChunks
	meta.Placement = initPlacement(meta.Placement, numChunks)
	meta.ChunkSize = chunkSize
	meta.Deleted = false

	if meta.slice.initialized {
		meta.slice = Slice{}
	}
	meta.placerMeta = nil

	return meta
}

func (m *Meta) close() {
	metaPool.Put(m)
}

func (m *Meta) ChunkKey(chunkId int) string {
	return fmt.Sprintf("%d@%s", chunkId, m.Key)
}

func (m *Meta) ChunkMkKey(chunkId int, lowLvlKey string) string {
	return fmt.Sprintf("%d@%s@%s", chunkId, m.Key, lowLvlKey)
}

type Placement []int

func initPlacement(p Placement, sz int) Placement {
	if p == nil && sz == 0 {
		return p
	} else if p == nil || cap(p) < sz {
		return make(Placement, sz)
	} else {
		p = p[:sz]
		if sz > 0 {
			copy(p, emptyPlacement[:sz])
		}
		return p
	}
}

func copyPlacement(p Placement, src Placement) Placement {
	sz := len(src)
	if p == nil || cap(p) < sz {
		p = make(Placement, sz)
	}
	copy(p, src)
	return p
}

func IsPlacementEmpty(p Placement) bool {
	return p == nil || len(p) == 0
}
