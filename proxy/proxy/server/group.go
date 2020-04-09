package server

import(
	"github.com/neboduus/infinicache/proxy/proxy/types"
	"github.com/neboduus/infinicache/proxy/proxy/lambdastore"
	"sync"
	"sync/atomic"
)

type Group struct {
	All         []*GroupInstance

	size        int
	sliceBase   uint64
}

type GroupInstance struct {
	types.LambdaDeployment
	group       *Group
	idx         int
}

func NewGroup(num int) *Group {
	return &Group{
		All: make([]*GroupInstance, num),
		size: num,
	}
}

func (g *Group) Len() int {
	return g.size
}

func (g *Group) InitMeta(meta *Meta, sliceSize int) *Meta {
	meta.slice.group = g
	meta.slice.size = sliceSize
	return meta
}

func (g *Group) Reserve(idx int, d types.LambdaDeployment) *GroupInstance {
	return &GroupInstance{ d, g, idx }
}

func (g *Group) Set(ins *GroupInstance) {
	switch ins.LambdaDeployment.(type) {
	case *lambdastore.Deployment:
		ins.LambdaDeployment = lambdastore.NewInstanceFromDeployment(ins.LambdaDeployment.(*lambdastore.Deployment))
	}
	g.All[ins.idx] = ins
}

func (g *Group) Validate(ins *GroupInstance) *GroupInstance {
	gins := g.All[ins.idx]
	if gins == nil {
		g.Set(ins)
	} else if gins != ins {
		gins.LambdaDeployment.(*lambdastore.Instance).Switch(ins)
	}

	return gins
}

func (g *Group) Instance(idx int) *lambdastore.Instance {
	return g.All[idx].LambdaDeployment.(*lambdastore.Instance)
}

func (g *Group) nextSlice(sliceSize int) int {
	return int((atomic.AddUint64(&g.sliceBase, uint64(sliceSize)) - uint64(sliceSize)) % uint64(g.size))
}

type Slice struct {
	once        sync.Once
	initialized bool
	group       *Group
	size        int
	base        int
}

func (s *Slice) GetIndex(idx int) int {
	s.once.Do(s.get)
	return (s.base + idx) % s.group.size
}

func (s *Slice) get() {
	s.base = s.group.nextSlice(s.size)
	s.initialized = true
}
