package lambdastore

import (
	"sync/atomic"
	"time"
)

type ChunkMeta struct {
	Key string
	Size uint64
	Hit bool
	Accessed time.Time

	prev *ChunkMeta
	next *ChunkMeta
}

// FULL = (Updates - SnapshotUpdates + SnapshotSize) / Bandwidth + (Term - SnapShotTerm + 1) * RTT
// INCREMENTAL = (Updates - LastUpdates) / Bandwidth + (Seq - LastSeq) * RTT
// FULL < INCREMENTAL
type Meta struct {
	// Sequence of the last confirmed log. Logs store by sequence.
	Term     uint64

	// Total transmission size for restoring all confirmed logs.
	Updates  uint64

	// Hash of the last confirmed log.
	Hash     string

	// Sequence of snapshot.
	SnapShotTerm    uint64

	// Total transmission size for restoring all confirmed logs from start to SnapShotSeq.
	SnapshotUpdates uint64

	// Total size of snapshot for transmission.
	SnapshotSize    uint64

	// TODO: implement clock LRU

	// Capacity of the instance.
	Capacity        uint64

	// Size of the instance.
	size            uint64

	chunks map[string]*ChunkMeta
	head ChunkMeta
	anchor *ChunkMeta
}

func (m *Meta) Size() uint64 {
	return atomic.LoadUint64(&m.size)
}

func (m *Meta) IncreaseSize(inc int64) uint64 {
	return atomic.AddUint64(&m.size, uint64(inc))
}

func (m *Meta) DecreaseSize(dec int64) uint64 {
	return atomic.AddUint64(&m.size, ^uint64(dec-1))
}
