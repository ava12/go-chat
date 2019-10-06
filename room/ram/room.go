package ram

import (
	"github.com/ava12/go-chat/room"
	"fmt"
	"sync"
)

type memRegistryRec struct {
	lock sync.RWMutex
	rooms map[int]*room.Entry
	lastId int
}

func NewRegistry () room.Registry {
	return &memRegistryRec {rooms: make(map[int]*room.Entry)}
}

func (mrr *memRegistryRec) ListRooms () []room.Entry {
	mrr.lock.RLock()
	defer mrr.lock.RUnlock()

	result := make([]room.Entry, 0, len(mrr.rooms))
	for _, entry := range mrr.rooms {
		result = append(result, *entry)
	}
	return result
}

func (mrr *memRegistryRec) CreateRoom (name string) (id int, e error) {
	mrr.lock.RLock()
	defer mrr.lock.RUnlock()

	for _, entry := range mrr.rooms {
		if entry.Name == name {
			return 0, fmt.Errorf("room \"%s\" already exists", name)
		}
	}

	mrr.lastId++
	mrr.rooms[mrr.lastId] = &room.Entry {mrr.lastId, name}
	return mrr.lastId, nil
}

func (mrr *memRegistryRec) Room (id int) (room.Entry, bool) {
	mrr.lock.RLock()
	defer mrr.lock.RUnlock()

	entry := mrr.rooms[id]
	if entry != nil {
		return *entry, true
	} else {
		return room.Entry {}, false
	}
}
