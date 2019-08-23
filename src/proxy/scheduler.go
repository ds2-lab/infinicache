package proxy

import(
	"errors"
	"fmt"
	"github.com/cornelk/hashmap"

	"github.com/wangaoone/LambdaObjectstore/src/proxy/types"
	"github.com/wangaoone/LambdaObjectstore/src/proxy/global"
	"github.com/wangaoone/LambdaObjectstore/src/proxy/lambdastore"
	"github.com/wangaoone/LambdaObjectstore/src/migrator"
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

func (s *Scheduler) GetForGroup(g *Group, idx int) *lambdastore.Instance {
	ins := g.Reserve(idx, lambdastore.NewInstanceFromDeployment(<-s.pool))
	s.actives.Set(ins.Id(), ins)
	g.Set(ins)
	return ins.LambdaDeployment.(*lambdastore.Instance)
}

func (s *Scheduler) ReserveForGroup(g *Group, idx int) (types.LambdaDeployment, error) {
	select {
	case item := <-s.pool:
		ins := g.Reserve(idx, item)
		s.actives.Set(ins.Id(), ins)
		return ins.LambdaDeployment, nil
	default:
		return nil, types.ErrNoSpareDeployment
	}
}

func (s *Scheduler) ReserveForInstance(insId uint64) (types.LambdaDeployment, error) {
	got, exists := s.actives.Get(insId)
	if !exists {
		return nil, errors.New(fmt.Sprintf("Instance %d not found.", insId))
	}

	ins := got.(*GroupInstance)
	return s.ReserveForGroup(ins.group, ins.idx)
}

func (s *Scheduler) Recycle(dp types.LambdaDeployment) {
	s.actives.Del(dp.Id())
	switch dp.(type) {
	case *lambdastore.Deployment:
		s.pool <- dp.(*lambdastore.Deployment)
	case *lambdastore.Instance:
		dp.(*lambdastore.Instance).Close()
		s.pool <- dp.(*lambdastore.Instance).Deployment
	}
}

func (s *Scheduler) Deployment(id uint64) (types.LambdaDeployment, bool) {
	ins, exists := s.actives.Get(id)
	if exists {
		return ins.(*GroupInstance).LambdaDeployment, exists
	} else {
		return nil, exists
	}
}

func (s *Scheduler) Instance(id uint64) (*lambdastore.Instance, bool) {
	got, exists := s.actives.Get(id)
	if !exists {
		return nil, exists
	}

	ins := got.(*GroupInstance)
	validated := ins.group.Validate(ins)
	if validated != ins {
		// Switched, recycle ins
		s.Recycle(ins.LambdaDeployment)
	}
	return validated.LambdaDeployment.(*lambdastore.Instance), exists
}

func (s *Scheduler) Clear(g *Group) {
	for item := range s.actives.Iter() {
		ins := item.Value.(*GroupInstance)
		if ins.group == g {
			s.Recycle(ins.LambdaDeployment)
		}
	}
}

func (s *Scheduler) ClearAll() {
	for item := range s.actives.Iter() {
		s.Recycle(item.Value.(*GroupInstance).LambdaDeployment)
	}
}

// MigrationScheduler implementations
func (s *Scheduler) StartMigrator(lambdaId uint64) (string, error) {
	m := migrator.New(global.BaseMigratorPort + int(lambdaId), true)
	err := m.Listen()
	if err != nil {
		return "", err
	}

	go m.Serve()

	return m.Addr, nil
}

func (s *Scheduler) GetDestination(lambdaId uint64) (types.LambdaDeployment, error) {
	return scheduler.ReserveForInstance(lambdaId)
}

func init() {
	scheduler = newScheduler()
	for i := 0; i < LambdaMaxDeployments; i++ {
		scheduler.pool <- lambdastore.NewDeployment(LambdaPrefix, uint64(i), false)
	}

	lambdastore.Registry = scheduler
	global.Migrator = scheduler
}
