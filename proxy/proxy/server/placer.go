package server

import (
	"fmt"
	"github.com/neboduus/infinicache/proxy/common/logger"
	"github.com/neboduus/infinicache/proxy/proxy/global"
	"strings"
	"sync"
	"time"
)

const (
	INIT_LRU_CAPACITY = 10000
)

type PlacerMeta struct {
	// Object management properties
	pos        [2]int // Positions on both primary and secondary array.
	visited    bool
	visitedAt  time.Time
	confirmed  []bool
	numConfirmed int
	swapMap    Placement // For decision from LRU
	evicts     *Meta
	suggestMap Placement // For decision from balancer
	once       *sync.Once
	action     MetaDoPostProcess
}

func newPlacerMeta(numChunks int) *PlacerMeta {
	return &PlacerMeta {
		confirmed: make([]bool, numChunks),
	}
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

func (pm *PlacerMeta) confirm(chunk int) {
	if !pm.confirmed[chunk] {
		pm.confirmed[chunk] = true
		pm.numConfirmed++
	}
}

func (pm *PlacerMeta) allConfirmed() bool {
	return pm.numConfirmed == len(pm.confirmed)
}

// Placer implements a Clock LRU for object eviction. Because objects (or metas) are constantly added to the system
// in partial (chunks), a object (newer) can be appended to the list normally, while a later chunk of the object requires an
// eviction of another object (older). In this case, we evict the older in whole based on Clock LRU algorithm, and set the
// original position of the newer to nil, which a compact operation is needed later. We use a secondary array for online compact.
type Placer struct {
	log       logger.ILogger
	store     *MetaStore // HashMap of Metas (objects)
	group     *Group
	objects   [2][]*Meta // We use a secondary array for online compact, the first element is reserved.
	cursor    int        // Cursor of primary array that indicate where the LRU checks.
	cursorBound int      // The largest index the cursor can reach in current iteration.
	primary   int
	secondary int
	mu        sync.RWMutex
}

func NewPlacer(store *MetaStore, group *Group) *Placer {
	placer := &Placer{
		log: &logger.ColorLogger{
			Prefix: "Placer ",
			Level:  global.Log.GetLevel(),
			Color:  true,
		},
		store:     store,
		group:     group,
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

	// Check if it is "evicted".
	if meta.Deleted {
		meta.placerMeta = nil
		meta.Deleted = false
	}

	// Usually this should always be false for SET operation, flag RESET if true.
	if meta.placerMeta != nil && meta.placerMeta.confirmed[chunk] {
		meta.Reset = true
		// No size update is required, reserved on setting.
		return meta, got, nil
	}

	// Initialize placerMeta if not.
	if meta.placerMeta == nil {
		meta.placerMeta = newPlacerMeta(len(meta.Placement))
	}

	// Check availability
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double check if it is evicted.
	if meta.Deleted {
		return meta, got, nil
	}

	// Check if a replacement decision has been made.
	if !IsPlacementEmpty(meta.placerMeta.swapMap) {
		meta.Placement[chunk] = meta.placerMeta.swapMap[chunk]
		meta.placerMeta.confirm(chunk)

		// No size update is required, reserved on eviction.

		return meta, got, nil
	}

	assigned := meta.slice.GetIndex(lambdaId)
	instance := p.group.Instance(assigned)
	if instance.Meta.Size() + uint64(meta.ChunkSize) < instance.Meta.Capacity {
		meta.Placement[chunk] = assigned
		meta.placerMeta.confirm(chunk)
		// If the object has not seen.
		if meta.placerMeta.pos[p.primary] == 0 {
			p.AddObject(meta)
		}
		// We can add size to instance safely, the allocated space is reserved for this chunk even set operation may fail.
		// This allow the client to reset the chunk without affecting the placement.
		size := instance.Meta.IncreaseSize(meta.ChunkSize)
		p.log.Debug("Lambda %d size updated: %d of %d (key:%d@%s, Δ:%d).",
								assigned, size, instance.Meta.Capacity, chunk, key, meta.ChunkSize)
		return meta, got, nil
	}

	// Try find a replacement
	// p.log.Warn("lambda %d overweight triggered by %d@%s, meta: %v", assigned, chunk, meta.Key, meta)
	// p.log.Info(p.dumpPlacer())
	for !p.NextAvailableObject(meta) {
		// p.log.Warn("lambda %d overweight triggered by %d@%s, meta: %v", assigned, chunk, meta.Key, meta)
		// p.log.Info(p.dumpPlacer())
	}
	// p.log.Debug("meta key is: %s, chunk is %d, evicted, evicted key: %s, placement: %v", meta.Key, chunk, meta.placerMeta.evicts.Key, meta.placerMeta.evicts.Placement)

	for i, tbe := range meta.placerMeta.swapMap {	// To be evicted
		instance := p.group.Instance(tbe)
		if !meta.placerMeta.confirmed[i] {
			// Confirmed chunk will not move

			// The size can be replaced safely, too.
			size := instance.Meta.IncreaseSize(meta.ChunkSize - meta.placerMeta.evicts.ChunkSize)
			p.log.Debug("Lambda %d size updated: %d of %d (key:%d@%s, evict:%d@%s, Δ:%d).",
									tbe, size, instance.Meta.Capacity, i, key, i, meta.placerMeta.evicts.Key,
									meta.ChunkSize - meta.placerMeta.evicts.ChunkSize)
		} else {
			size := instance.Meta.DecreaseSize(meta.placerMeta.evicts.ChunkSize)
			p.log.Debug("Lambda %d size updated: %d of %d (evict:%d@%s, Δ:%d).",
									tbe, size, instance.Meta.Capacity, i, meta.placerMeta.evicts.Key,
									-meta.placerMeta.evicts.ChunkSize)
		}
	}

	meta.Placement[chunk] = meta.placerMeta.swapMap[chunk]
	meta.placerMeta.confirm(chunk)
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

	p.log.Debug("len %d dChunk %d", len(meta.placerMeta.confirmed), chunk)
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
	meta.placerMeta.visited = true
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
			p.objects[p.secondary] = make([]*Meta, 1, 2*len(p.objects[p.primary]))
		} else {
			p.objects[p.secondary] = p.objects[p.secondary][:1] // Alwarys append from the 2nd position.
		}
		p.cursorBound = len(p.objects[p.primary]) // CusorBound is fixed once start iteration.
		p.cursor = 1
	}

	found := false
	for _, m := range p.objects[p.primary][p.cursor:p.cursorBound] {
		if m == nil {
			// skip empty slot.
			p.cursor++
			continue
		}

		if m == meta {
			// Ignore meta itself
		} else if m.placerMeta.visited && m.placerMeta.allConfirmed() {
			// Only switch to unvisited for complete object.
			m.placerMeta.visited = false
		} else if !m.placerMeta.visited {
			// Found candidate
			m.Deleted = true
			// Don't reset placerMeta here, reset on recover object.
			// m.placerMeta = nil
			meta.placerMeta.swapMap = copyPlacement(meta.placerMeta.swapMap, m.Placement)
			meta.placerMeta.evicts = m

			p.objects[p.primary][meta.placerMeta.pos[p.primary]] = nil // unset old position
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

	// Reach end of the primary array(cursorBound), reset cursor and switch arraies.
	if !found {
		// Add new objects to the secondary array for compact purpose.
		if len(p.objects[p.primary]) > p.cursor {
			for _, m := range p.objects[p.primary][p.cursor:] {
				if m == nil {
					continue
				}

				m.placerMeta.pos[p.secondary] = len(p.objects[p.secondary])
				p.objects[p.secondary] = append(p.objects[p.secondary], m)
			}
		}
		// Reset cursor and switch
		p.cursor = 0
		p.primary, p.secondary = p.secondary, p.primary
	}

	return found
}

func (p *Placer) dumpPlacer(args ...bool) string {
	if len(args) > 0 && args[0] {
		return p.dump(p.objects[p.secondary])
	} else {
		return p.dump(p.objects[p.primary])
	}
}

func (p *Placer) dump(metas []*Meta) string {
	if metas == nil || len(metas) < 1 {
		return ""
	}

	elem := make([]string, len(metas)-1)
	for i, meta := range metas[1:] {
		if meta == nil {
			elem[i] = "nil"
			continue
		}

		visited := 0
		if meta.placerMeta.visited {
			visited = 1
		}
		elem[i] = fmt.Sprintf("%s-%d", meta.Key, visited)
	}
	return strings.Join(elem, ",")
}
