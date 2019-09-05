package lifetime

import (
	"net"
	"sync"
	"time"

	"github.com/wangaoone/LambdaObjectstore/src/LambdaStore/migrator"
)

var (
	session *Session
	mu      sync.RWMutex
)

type Session struct {
	Requests  int
	Clear     sync.WaitGroup
	Migrator  *migrator.Client
	Timeout   *Timeout
	Connection net.Conn

	done      chan struct{}
}

func GetSession() *Session {
	mu.Lock()
	defer mu.Unlock()


	if session == nil {
		session = &Session{ done: make(chan struct{}) }
		session.Timeout = NewTimeout(session, time.Duration(TICK_ERROR_EXTEND))
	}
	return session
}

func ClearSession() {
	mu.Lock()
	defer mu.Unlock()

	session = nil
}

func (s *Session) WaitDone() <-chan struct{} {
	return s.done
}

func (s *Session) Done() {
	mu.Lock()
	defer mu.Unlock()

	s.DoneLocked()
}

func (s *Session) IsDone() bool {
	mu.RLock()
	defer mu.RUnlock()

	if s.done == nil {
		return true
	}

	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

func (s *Session) Lock() {
	mu.Lock()
}

func (s *Session) Unlock() {
	mu.Unlock()
}

func (s *Session) DoneLocked() {
	select {
	case <-s.done:
		// closed
	default:
		close(s.done)
	}
}

func (s *Session) resetDoneLocked() {
	if s.done == nil {
		s.done = make(chan struct{})
	}
}
