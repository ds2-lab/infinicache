package proxy

import(
	"github.com/cornelk/hashmap"

	"github.com/wangaoone/LambdaObjectstore/src/proxy/types"
	"github.com/wangaoone/LambdaObjectstore/src/proxy/lambdastore"
)

const DEP_STATUS_POOLED = 0
const DEP_STATUS_ACTIVE = 1
const DEP_STATUS_ACTIVATING = 2

var (
	scheduler    *Scheduler
)

type Scheduler struct {
	pool           chan *lambdastore.Deployment
	actives        *hashmap.HashMap
}

func newScheduler() *Scheduler {
	return &Scheduler{
		pool: make(chan *lambdastore.Deployment, LambdaMaxDeployments + 1), // Allocate extra 1 buffer to avoid blocking
		actives: hashmap.New(NumLambdaClusters),
	}
}

func (d *Scheduler) GetForGroup(g *Group, idx int) *lambdastore.Instance {
	ins := g.Reserve(idx, lambdastore.NewInstanceFromDeployment(<-d.pool))
	d.actives.Set(ins.Id(), ins)
	g.Set(ins)
	return ins.LambdaDeployment.(*lambdastore.Instance)
}

func (d *Scheduler) ReserveForGroup(g *Group, idx int) types.LambdaDeployment {
	ins := g.Reserve(idx, <-d.pool)
	d.actives.Set(ins.Id(), ins)
	return ins.LambdaDeployment
}

func (d *Scheduler) Recycle(dp types.LambdaDeployment) {
	d.actives.Del(dp.Id())
	switch dp.(type) {
	case *lambdastore.Deployment:
		d.pool <- dp.(*lambdastore.Deployment)
	case *lambdastore.Instance:
		dp.(*lambdastore.Instance).Close()
		d.pool <- dp.(*lambdastore.Instance).Deployment
	}
}

func (d *Scheduler) Deployment(id uint64) (types.LambdaDeployment, bool) {
	ins, exists := d.actives.Get(id)
	if exists {
		return ins.(*GroupInstance).LambdaDeployment, exists
	} else {
		return nil, exists
	}
}

func (d *Scheduler) Instance(id uint64) (*lambdastore.Instance, bool) {
	got, exists := d.actives.Get(id)
	if !exists {
		return nil, exists
	}

	ins := got.(*GroupInstance)
	validated := ins.group.Validate(ins)
	if validated != ins {
		// Switched, recycle ins
		d.Recycle(ins.LambdaDeployment)
	}
	return validated.LambdaDeployment.(*lambdastore.Instance), exists
}

func (d *Scheduler) Clear(g *Group) {
	for item := range d.actives.Iter() {
		ins := item.Value.(*GroupInstance)
		if ins.group == g {
			d.Recycle(ins.LambdaDeployment)
		}
	}
}

func (d *Scheduler) ClearAll() {
	for item := range d.actives.Iter() {
		d.Recycle(item.Value.(*GroupInstance).LambdaDeployment)
	}
}

func init() {
	scheduler = newScheduler()
	lambdastore.Registry = scheduler
	for i := 0; i < LambdaMaxDeployments; i++ {
		scheduler.pool <- lambdastore.NewDeployment(LambdaPrefix, uint64(i), false)
	}
}
