package lifetime

import (
	"time"
)

type Lifetime struct {
	birthtime time.Time
	alive     bool
	expected  time.Duration
}

func New(expected time.Duration) *Lifetime{
	return &Lifetime{
		birthtime: time.Now(),
		alive: true,
		expected: expected,
	}
}

func (l *Lifetime) Id() int64 {
	return l.birthtime.UnixNano()
}

func (l *Lifetime) Reborn() {
	l.birthtime = time.Now()
	l.alive = true
}

func (l *Lifetime) RebornIfDead() {
	if !l.alive {
		l.Reborn()
	}
}

func (l *Lifetime) IsTimeUp() bool {
	return int64(time.Since(l.birthtime)) >= int64(l.expected)
}

func (l *Lifetime) Rest() {
	l.alive = false
}
