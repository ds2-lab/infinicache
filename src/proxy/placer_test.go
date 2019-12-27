package proxy

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"strconv"
	"strings"
	"sync"
//	"log"

//	"github.com/wangaoone/LambdaObjectstore/src/proxy"
)

var container []*Meta

func newTestMeta(i int) *Meta {
	return &Meta{
		Key: strconv.Itoa(i),
		Placement: []int{i},
		placerMeta: &PlacerMeta{},
	}
}

func initPlacer() *Placer {
	container = make([]*Meta, 0, 15)
	placer := NewPlacer(nil, nil)

	for i := 0; i < 10; i++ {
		container = append(container, newTestMeta(i))
		placer.AddObject(container[i])
	}
	for i := 0; i < 4; i++ {
		placer.TouchObject(container[i])
	}
	for i := 5; i < 10; i++ {
		placer.TouchObject(container[i])
	}
	return placer
}

func dumpPlacer(p *Placer, args ...bool) string {
	if len(args) > 0 && args[0] {
		return dump(p.objects[p.secondary])
	} else {
		return dump(p.objects[p.primary])
	}
}

func dump(metas []*Meta) string {
	if metas == nil || len(metas) < 1 {
		return ""
	}

	elem := make([]string, len(metas) - 1)
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

var _ = Describe("Placer", func() {
	It("should visited be initialized with false", func() {
		placer := initPlacer()
		Expect(dumpPlacer(placer)).To(Equal("0-1,1-1,2-1,3-1,4-0,5-1,6-1,7-1,8-1,9-1"))
		Expect(dumpPlacer(placer, true)).To(Equal(""))
	})

	It("should replace the unvisited object", func() {
		placer := initPlacer()
		idx := len(container)
		container = append(container, newTestMeta(idx))

		found := placer.NextAvailableObject(container[idx])
		Expect(found).To(Equal(true))
		Expect(dumpPlacer(placer)).To(Equal(fmt.Sprintf("0-0,1-0,2-0,3-0,%d-1,5-1,6-1,7-1,8-1,9-1", idx)))
		Expect(dumpPlacer(placer, true)).To(Equal(fmt.Sprintf("0-0,1-0,2-0,3-0,%d-1", idx)))
		Expect(container[idx].placerMeta.pos).To(Equal([2]int{5, 5}))
		Expect(container[idx].placerMeta.swapMap).To(Equal(container[4].Placement))
	})

	It("should replace the unvisited object even the newer has been appended to the list", func() {
		placer := initPlacer()
		idx := len(container)
		container = append(container, newTestMeta(idx))

		placer.AddObject(container[idx])
		Expect(dumpPlacer(placer)).To(Equal(fmt.Sprintf("0-1,1-1,2-1,3-1,4-0,5-1,6-1,7-1,8-1,9-1,%d-0", idx)))
		Expect(container[idx].placerMeta.pos).To(Equal([2]int{idx + 1, 0}))

		found := placer.NextAvailableObject(container[idx])
		Expect(found).To(Equal(true))
		Expect(dumpPlacer(placer)).To(Equal(fmt.Sprintf("0-0,1-0,2-0,3-0,%d-1,5-1,6-1,7-1,8-1,9-1,nil", idx)))
		Expect(dumpPlacer(placer, true)).To(Equal(fmt.Sprintf("0-0,1-0,2-0,3-0,%d-1", idx)))
		Expect(container[idx].placerMeta.pos).To(Equal([2]int{5, 5}))
		Expect(container[idx].placerMeta.swapMap).To(Equal(container[4].Placement))
	})

	It("should a second call and compact works if the first call failed", func() {
		placer := initPlacer()
		idx := len(container)
		container = append(container, newTestMeta(idx), newTestMeta(idx + 1))

		placer.AddObject(container[idx])
		placer.NextAvailableObject(container[idx])

		found := placer.NextAvailableObject(container[idx + 1])
		Expect(found).To(Equal(false))
		Expect(dumpPlacer(placer)).To(Equal(fmt.Sprintf("0-0,1-0,2-0,3-0,%d-1,5-0,6-0,7-0,8-0,9-0", idx)))
		Expect(dumpPlacer(placer, true)).To(Equal(fmt.Sprintf("0-0,1-0,2-0,3-0,%d-1,5-0,6-0,7-0,8-0,9-0,nil", idx)))

		found = placer.NextAvailableObject(container[idx + 1])
		Expect(found).To(Equal(true))
		Expect(dumpPlacer(placer)).To(Equal(fmt.Sprintf("%d-1,1-0,2-0,3-0,%d-1,5-0,6-0,7-0,8-0,9-0", idx + 1, idx)))
		Expect(dumpPlacer(placer, true)).To(Equal(fmt.Sprintf("%d-1", idx + 1)))
		Expect(container[idx + 1].placerMeta.pos).To(Equal([2]int{1, 1}))
		Expect(container[idx + 1].placerMeta.swapMap).To(Equal(container[0].Placement))
	})

	It("should post process callback works", func() {
		var called string
		cb := func(meta *Meta) {
			called = meta.Key
		}

		meta := newTestMeta(1)
		meta.placerMeta.evicts = newTestMeta(2)
		meta.placerMeta.once = &sync.Once{}
		meta.placerMeta.postProcess(cb)

		Expect(called).To(Equal("2"))
	})

})
