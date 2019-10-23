package ram

import (
	"net/http"
	"sync"
	"strings"
)

type UserEntry struct {
	Id int `json:"id"`
	Name string `json:"name"`
}

type Registry struct {
	lock sync.RWMutex
	names map[int]string
	ids map[string]int
	lastId int
}

func NewRegistry () *Registry {
	return &Registry {names: make(map[int]string), ids: make(map[string]int)}
}

func (r *Registry) User (id int) (interface{}, bool) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	name, has := r.names[id]
	if has {
		return UserEntry {Id: id, Name: name}, true
	} else {
		return UserEntry {}, false
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

func (r *Registry) Login (w http.ResponseWriter, re *http.Request) (int, interface {}, error) {
	name := strings.TrimSpace(re.PostFormValue("name"))
	uid := r.UserIdByName(name)
	if uid == 0 {
		uid = r.AddUser(name)
	}
	return uid, &UserEntry {uid, name}, nil
}
