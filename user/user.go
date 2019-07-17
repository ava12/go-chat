package user

import (
	"github.com/ava12/go-chat/proto"
	"sync"
)

type Registry struct {
	lock sync.RWMutex
	names map[int]string
	ids map[string]int
	lastId int
}

func NewRegistry () *Registry {
	return &Registry {names: make(map[int]string), ids: make(map[string]int)}
}

func (r *Registry) User (id int) (proto.UserEntry, bool) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	name, has := r.names[id]
	if has {
		return proto.UserEntry {Id: id, Name: name}, true
	} else {
		return proto.UserEntry {}, false
	}
}

func (r *Registry) AddUser (name string) int {
	r.lock.Lock()
	defer r.lock.Unlock()

	id := r.ids[name]
	if id > 0 {
		return id
	}

	r.lastId++
	id = r.lastId
	r.names[id] = name
	r.ids[name] = id
	return id
}

func (r *Registry) UserIdByName (name string) int {
	r.lock.RLock()
	defer r.lock.RUnlock()

	return r.ids[name]
}
