package lifetime

import (
	"net"
	"sync"
	"time"

	"github.com/neboduus/infinicache/node/lambda/migrator"
)

var (
	session *Session
	mu      sync.RWMutex
)

type Session struct {
	Id        string
	Requests  int
	Clear     sync.WaitGroup
	Migrator  *migrator.Client
	Timeout   *Timeout
	Connection net.Conn

	done      chan struct{}
}

func GetOrCreateSession() *Session {
	mu.Lock()
	defer mu.Unlock()

	if session == nil {
		session = &Session{ done: make(chan struct{}) }
		session.Timeout = NewTimeout(session, time.Duration(TICK_ERROR_EXTEND))
	}
	return session
}

func GetSession() *Session {
	mu.RLock()
	defer mu.RUnlock()

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

	return s.isDoneLocked()
}

func (s *Session) Lock() {
	mu.Lock()
}

func (s *Session) Unlock() {
	mu.Unlock()
}

func (s *Session) IsMigrating() bool {
	return s.Migrator != nil
}

func (s *Session) isDoneLocked() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

func (s *Session) DoneLocked() {
	select {
	case <-s.done:
		// closed
	default:
		close(s.done)
	}
}
