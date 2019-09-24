package session

import (
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

const DefaultTtl = time.Hour

type Session struct {
	ttl       int64 // seconds
	timestamp int64 // seconds
	id        string
	userId    int
}

func newSession(userId int, ttl time.Duration) *Session {
	return &Session{int64(ttl / time.Second), time.Now().Unix(), strconv.FormatInt(rand.Int63(), 10), userId}
}

func (s *Session) Id() string {
	return s.id
}

func (s *Session) UserId() int {
	return s.userId
}

func (s *Session) Ttl() int64 {
	return atomic.LoadInt64(&s.ttl)
}

func (s *Session) Touch() {
	atomic.StoreInt64(&s.timestamp, time.Now().Unix())
}

func (s *Session) Expired() bool {
	ts := atomic.LoadInt64(&s.timestamp)
	return (ts+s.ttl < time.Now().Unix())
}

type Registry struct {
	lock     sync.RWMutex
	sessions map[string]*Session
	ttl      time.Duration
}

func NewRegistry() *Registry {
	return &Registry{sessions: make(map[string]*Session), ttl: DefaultTtl}
}

func (r *Registry) Session(id string) *Session {
	r.lock.RLock()
	defer r.lock.RUnlock()

	result := r.sessions[id]
	if result == nil || result.Expired() {
		return nil
	}

	result.Touch()
	return result
}

func (r *Registry) Touch(id string) bool {
	r.lock.RLock()
	defer r.lock.RUnlock()

	s := r.sessions[id]
	if s != nil {
		s.Touch()
		return true
	} else {
		return false
	}
}

func (r *Registry) NewSession(userId int) *Session {
	r.lock.Lock()
	defer r.lock.Unlock()

	var result *Session
	for {
		result = newSession(userId, DefaultTtl)
		if r.sessions[result.id] == nil {
			break
		}
	}

	r.sessions[result.id] = result
	return result
}

func (r *Registry) Sweep() {
	r.lock.Lock()
	defer r.lock.Unlock()

	for id, s := range r.sessions {
		if !s.Expired() {
			continue
		}

		delete(r.sessions, id)
	}
}

func (r *Registry) Delete(id string) {
	r.lock.Lock()
	defer r.lock.Unlock()

	delete(r.sessions, id)
}
