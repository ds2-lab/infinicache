package lambdastore

import (
	"fmt"
	"github.com/neboduus/infinicache/proxy/common/logger"

	"github.com/neboduus/infinicache/proxy/proxy/types"
)

type Deployment struct {
	name      string
	id        uint64
	replica   bool
	log       logger.ILogger
}

func NewDeployment(name string, id uint64, replica bool) *Deployment {
	if !replica {
		name = fmt.Sprintf("%s%d", name, id)
	}
	return &Deployment{
		name:      name,
		id:        id,
		replica:   replica,
		log:       logger.NilLogger,
	}
}

func (d *Deployment) Name() string {
	return d.name
}

func (d *Deployment) Id() uint64 {
	return d.id
}

func (d *Deployment) Reset(new types.LambdaDeployment, old types.LambdaDeployment) {
	if old != nil {
		old.Reset(d, nil)
	}

	d.name = new.Name()
	d.id = new.Id()
	switch d.log.(type) {
	case *logger.ColorLogger:
		log := d.log.(*logger.ColorLogger)
		d.log = &logger.ColorLogger{
			Prefix: fmt.Sprintf("%s ", d.name),
			Level:  log.GetLevel(),
			Color:  log.Color,
		}
	}
}
