package proxy

import (
	"sync"
	"time"
)

const (
	INIT_LRU_CAPACITY = 10000
)

type PlacerMeta struct {
	// Object management properties
	pos         [2]int        // Positions on both primary and secondary array.
	visited     bool
	visitedAt   time.Time
	confirmed   []bool
	swapMap     Placement     // For decision from LRU
	evicts      *Meta
	suggestMap  Placement     // For decision from balancer
	once        *sync.Once
	action      MetaDoPostProcess
}

func (pm *PlacerMeta) postProcess(action MetaDoPostProcess) {
	if pm.once == nil {
		return
	}
	pm.action = action
	pm.once.Do(pm.doPostProcess)
}

func (pm *PlacerMeta) doPostProcess() {
	pm.action(pm.evicts)
	pm.evicts = nil
}

// Placer implements a Clock LRU for object eviction. Because objects (or metas) are constantly added to the system
// in partial (chunks), a object (newer) can be appended to the list normally, while a later chunk of the object requires an
// eviction of another object (older). In this case, we evict the older in whole based on Clock LRU algorithm, and set the
// original position of the newer to nil, which a compact operation is needed later. We use a secondary array for online compact.
type Placer struct {
	store       *MetaStore
	group       *Group
	objects     [2][]*Meta   // We use a secondary array for online compact, the first element is reserved.
	cursor      int          // Cursor of primary array that indicate where the LRU checks.
	primary     int
	secondary   int
	mu          sync.RWMutex
}

func NewPlacer(store *MetaStore, group *Group) *Placer {
	placer := &Placer{
		store: store,
		group: group,
		secondary: 1,
	}
	placer.objects[0] = make([]*Meta, 1, INIT_LRU_CAPACITY)
	return placer
}

func (p *Placer) NewMeta(key string, sliceSize int, numChunks int, chunk int, lambdaId int, chunkSize int64) *Meta {
	meta := NewMeta(key, numChunks, chunkSize)
	p.group.InitMeta(meta, sliceSize)
	meta.Placement[chunk] = lambdaId
	meta.lastChunk = chunk
	return meta
}

// NewMeta will remap idx according to following logic:
// 0. If an LRU relocation is present, remap according to "chunk" in relocation array.
// 1. Base on the size of slice, remap to a instance in the group.
// 2. If target instance is full, request an LRU relocation and restart from 0.
// 3. If no Balancer relocation is available, request one.
// 4. Remap to smaller "Size" of instance between target instance and remapped instance according to "chunk" in
//    relocation array.
func (p *Placer) GetOrInsert(key string, newMeta *Meta) (*Meta, bool, MetaPostProcess) {
	chunk := newMeta.lastChunk
	lambdaId := newMeta.Placement[chunk]

	meta, got, _ := p.store.GetOrInsert(key, newMeta)
	if got {
		newMeta.close()
	}

	meta.mu.Lock()
	defer meta.mu.Unlock()

	if meta.placerMeta != nil && meta.placerMeta.confirmed[chunk] {
		return meta, got, nil
	}

	if meta.placerMeta == nil {
		meta.placerMeta = &PlacerMeta{
			confirmed: make([]bool, len(meta.Placement)),
		}
	}

	// Check availability
	p.mu.Lock()
	defer p.mu.Unlock()

	assigned := meta.slice.GetIndex(lambdaId)
	instance := p.group.Instance(assigned)
	if instance.Meta.Size + uint64(meta.ChunkSize) < instance.Meta.Capacity {
		meta.Placement[chunk] = assigned
		meta.placerMeta.confirmed[chunk] = true
		if meta.placerMeta.pos[p.primary] == 0 {
			p.AddObject(meta)
		}
		return meta, got, nil
	}

	// Check if a replacement decision has been made.
	if !IsPlacementEmpty(meta.placerMeta.swapMap) {
		meta.Placement[chunk] = meta.placerMeta.swapMap[chunk]
		meta.placerMeta.confirmed[chunk] = true
		return meta, got, nil
	}

	// Try find a replacement
	for !p.NextAvailableObject(meta) {}

	meta.Placement[chunk] = meta.placerMeta.swapMap[chunk]
	meta.placerMeta.confirmed[chunk] = true
	meta.placerMeta.once = &sync.Once{}
	return meta, got, meta.placerMeta.postProcess
}

func (p *Placer) Get(key string, chunk int) (*Meta, bool) {
	meta, ok := p.store.Get(key)
	if !ok {
		return nil, ok
	}

	meta.mu.Lock()
	defer meta.mu.Unlock()

	if meta.Deleted {
		return meta, ok
	}

	if meta.placerMeta == nil || !meta.placerMeta.confirmed[chunk] {
		return nil, false
	}

	// Ensure availability
	p.mu.Lock()
	defer p.mu.Unlock()

	// Object may be evicted just before locking.
	if meta.Deleted {
		return meta, ok
	}

	p.TouchObject(meta)
	return meta, ok
}

// Object management implementation: Clock LRU
func (p *Placer) AddObject(meta *Meta) {
	meta.placerMeta.pos[p.primary] = len(p.objects[p.primary])
	// meta.placerMeta.visited = false    // For new object in list, visited is set to false to avoid Useless First Round,
	meta.placerMeta.visitedAt = time.Now()

	p.objects[p.primary] = append(p.objects[p.primary], meta)
}

func (p *Placer) TouchObject(meta *Meta) {
	meta.placerMeta.visited = true
	meta.placerMeta.visitedAt = time.Now()
}

func (p *Placer) NextAvailableObject(meta *Meta) bool {
	// Position 0 is reserved, cursor iterates from 1
	if p.cursor == 0 {
		if p.objects[p.secondary] == nil || cap(p.objects[p.secondary]) < len(p.objects[p.primary]) {
			p.objects[p.secondary] = make([]*Meta, 1, 2 * len(p.objects[p.primary]))
		} else {
			p.objects[p.secondary] = p.objects[p.secondary][:1] // Alwarys append from the 2nd position.
		}
		p.cursor = 1
	}

	found := false
	for _, m := range p.objects[p.primary][p.cursor:] {
		if m == nil {
			// skip empty slot.
			p.cursor++
			continue
		}

		if m.placerMeta.visited {
			m.placerMeta.visited = false
		} else {
			// Found
			m.Deleted = true
			m.placerMeta = nil
			meta.placerMeta.swapMap = copyPlacement(meta.placerMeta.swapMap, m.Placement)
			meta.placerMeta.evicts = m

			p.objects[p.primary][meta.placerMeta.pos[p.primary]] = nil  // unset old position
			meta.placerMeta.pos[p.primary] = p.cursor
			p.objects[p.primary][p.cursor] = meta // replace
			m = meta

			m.placerMeta.visited = true
			m.placerMeta.visitedAt = time.Now()
			found = true
		}

		// Add current object to the secondary array for compact purpose.
		m.placerMeta.pos[p.secondary] = len(p.objects[p.secondary])
		p.objects[p.secondary] = append(p.objects[p.secondary], m)
		p.cursor++

		if found {
			break
		}
	}

	if !found {
		// Reach end of the primary array, reset cursor and switch arraies.
		p.cursor = 0
		p.primary, p.secondary = p.secondary, p.primary
	}

	return found
}
