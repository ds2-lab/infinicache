package server

import(
	"errors"
	"fmt"
	"github.com/cornelk/hashmap"
	"strconv"

	"github.com/neboduus/infinicache/proxy/proxy/types"
	"github.com/neboduus/infinicache/proxy/proxy/global"
	"github.com/neboduus/infinicache/proxy/proxy/lambdastore"
	"github.com/neboduus/infinicache/proxy/migrator"
)

const DEP_STATUS_POOLED = 0
const DEP_STATUS_ACTIVE = 1
const DEP_STATUS_ACTIVATING = 2
const IN_DEPLOYMENT_MIGRATION = true

var (
	scheduler    *Scheduler
)

type Scheduler struct {
	pool           chan *lambdastore.Deployment
	actives        *hashmap.HashMap
}

func NewScheduler(numCluster int, numDeployment int) *Scheduler {
	s := &Scheduler{
		pool: make(chan *lambdastore.Deployment, numDeployment + 1), // Allocate extra 1 buffer to avoid blocking
		actives: hashmap.New(uintptr(numCluster)),
	}

	for _, address := range LambdaAddresses {
		s.pool <- lambdastore.NewDeployment(LambdaPrefix, uint64(findIndexFromStaticName(address)), false, address)
	}
	return s
}

func newScheduler() *Scheduler {
	return NewScheduler(NumLambdaClusters, LambdaMaxDeployments)
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
	if IN_DEPLOYMENT_MIGRATION {
		return ins.LambdaDeployment, nil
	} else {
		return s.ReserveForGroup(ins.group, ins.idx)
	}
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
		// Switch keys
		s.actives.Set(validated.Id(), validated)
		s.actives.Set(ins.Id(), ins)
		// Recycle ins
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

	lambdastore.Registry = scheduler
	global.Migrator = scheduler
}

func CleanUpScheduler() {
	scheduler.ClearAll()
	scheduler = nil

	migrator.CleanUp()
}

func findIndexFromStaticName(address string) int {
	var a string
	var b int
	for j, char := range address {
		if j < 24 {
			continue
		}
		if j == 24 {
			a = string(char)
		}
		if j == 25 {
			c, e := strconv.Atoi(string(char))
			b = c
			if e == nil {
				a = fmt.Sprintf("%s%d", a, b)
			}
			break
		}
	}
	i, _ := strconv.Atoi(a)
	return i
}