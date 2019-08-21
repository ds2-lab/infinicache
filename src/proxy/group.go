package proxy

import(
	"github.com/wangaoone/LambdaObjectstore/src/proxy/types"
	"github.com/wangaoone/LambdaObjectstore/src/proxy/lambdastore"
)

type Group struct {
	All []*GroupInstance
}

type GroupInstance struct {
	types.LambdaDeployment
	group       *Group
	idx         int
}

func NewGroup(num int) *Group {
	return &Group{ make([]*GroupInstance, num) }
}

func (g *Group) Len() int {
	return len(g.All)
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
