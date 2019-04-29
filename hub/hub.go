package hub

import (
	"time"
	"errors"
	"sync"
)

type Conn interface {
	Id () int
	UserId () int
	NewMessage (m *MessageEntry)
	UpdateMessage (m *MessageEntry)
	Notice (data interface {})
}

type MessageEntry struct {
	RoomId, MessageId, UserId int
	Timestamp int
	Data interface {}
}

type MessageList []*MessageEntry

const (
	defaultSenders = 10
	defaultFlushDelay = 30 * time.Second
	defaultFlushItems = 20
	defaultFlushThreshold = 50
)

const taskQueueLen = 10

type Hub interface {
	SetSenders (count int)
	SetFlushDelay (delay time.Duration)
	SetFlushItems (count int)
	SetFlushThreshold (count int)

	Start ()
	Stop ()

	Connect (c Conn) error
	Disconnect (connId int)
	IsConnected (connId int) bool
	Connection (connId int) Conn

	NewRoom (roomId, lastMessageId int, userIds []int)
	DeleteRoom (roomId int)
	EnterRoom (userId, roomId int) error
	LeaveRoom (userId, roomId int)

	NewMessage (connId, roomId int, data interface {}) (messageId int, e error)
	UpdateMessage (roomId, messageId int, data interface {}) error
	ConnNotice (connId int, data interface {}) error
	UserNotice (userId int, data interface {}) error
	RoomNotice (roomId int, data interface {}) error
	GlobalNotice (data interface {}) error

	Messages (userId, roomId, firstId, count int) (MessageList, error)
	UserRoomIds (userId int) []int
	IsInRoom (userId, roomId int) bool
	OnlineUserIds () []int
	RoomUserIds (roomId int) []int
	UserConnIds (userId int) []int
	UserIsConnected (userId int) bool
}


type MessageStorage interface {
	Save (m MessageList) error
	List (roomId, firstId, count int) (MessageList, error)
	Update (roomId, messageId int, data interface {}) (bool, error)
}

type memStorageRec struct {
	lock sync.RWMutex
	rooms map[int]MessageList
}

func NewMemStorage () MessageStorage {
	return &memStorageRec {rooms: make(map[int]MessageList)}
}

func (msr *memStorageRec) Save (messages MessageList) error {
	msr.lock.Lock()
	defer msr.lock.Unlock()

	for _, message := range messages {
		msr.rooms[message.RoomId] = append(msr.rooms[message.RoomId], message)
	}

	return nil
}

func (msr *memStorageRec) List (roomId, firstId, count int) (MessageList, error) {
	msr.lock.RLock()
	defer msr.lock.RUnlock()

	firstIndex := firstId - 1
	if firstIndex < 0 {
		firstIndex = 0
	}
	lastIndex := firstIndex + count
	return msr.rooms[roomId][firstIndex:lastIndex], nil
}

func (msr *memStorageRec) Update (roomId, messageId int, data interface {}) (bool, error) {
	msr.lock.Lock()
	defer msr.lock.Unlock()

	index := messageId - 1
	messages := msr.rooms[roomId]
	if index >= len(messages) {
		return false, nil
	}

	msr.rooms[roomId][index].Data = data
	return true, nil
}


type roomRec struct {
	UserIds []int
	LastMessageId int
}

const (
	connTarget = iota
	userTarget
	roomTarget
	globalTarget
)

type sendFunc func (c Conn)

type taskRec struct {
	Target, Id int
	Func sendFunc
}

type parcelRec struct {
	Conn Conn
	Func sendFunc
}

type hubRec struct {
	storage MessageStorage

	flushLock5 sync.Mutex
	flushDelay time.Duration
	flushItems, flushThreshold int
	flushTimer *time.Timer

	messageLock10 sync.RWMutex
	messages MessageList

	connLock20 sync.RWMutex
	conns map[int]Conn
	userConnIds map[int][]int

	roomLock30 sync.RWMutex
	rooms map[int]*roomRec

	taskQueue chan *taskRec
	senderCnt int
	parcelQueue chan *parcelRec
	senderGroup sync.WaitGroup

	stopSignal chan bool
	isRunning bool
}

var Stopped error = errors.New("hub is stopped")

func New (storage MessageStorage) Hub {
	if storage == nil {
		panic("no message storage")
	}

	result := &hubRec {
		storage: storage,
		flushDelay: defaultFlushDelay,
		flushItems: defaultFlushItems,
		flushThreshold: defaultFlushThreshold,
		conns: make(map[int]Conn),
		userConnIds: make(map[int][]int),
		rooms: make(map[int]*roomRec),
		messages: make(MessageList, 0),
		senderCnt: defaultSenders,
	}

	return result
}

func (h *hubRec) flush () {
	h.flushLock5.Lock()
	defer h.flushLock5.Unlock()
	h.messageLock10.RLock()
	defer h.messageLock10.RUnlock()

	var (i int; e error)
	cnt := len(h.messages)
	if h.flushItems > 0 && cnt > 0 {
		for i = 0; i < cnt; i += h.flushItems {
			e = h.storage.Save(h.messages[i:i + h.flushItems])
			if e != nil {
				break
			}
		}

		h.messages = h.messages[i:]
		if len(h.messages) > h.flushThreshold {
			panic(e)
		}
	}

	if h.flushTimer != nil {
		h.flushTimer.Stop()
		h.flushTimer.Reset(h.flushDelay)
	}
}

func (h *hubRec) SetSenders (count int) {
	h.senderCnt = count
}

func (h *hubRec) SetFlushDelay (delay time.Duration) {
	h.flushLock5.Lock()
	defer h.flushLock5.Unlock()

	if delay <= 0 {
		delay = 0
		if h.flushTimer != nil {
			h.flushTimer.Stop()
			h.flushTimer = nil
		}
	} else if h.isRunning && h.flushDelay <= 0 {
		h.flushTimer = time.AfterFunc(delay, h.flush)
	}

	h.flushDelay = delay
}

func (h *hubRec) SetFlushItems (count int) {
	h.flushLock5.Lock()
	defer h.flushLock5.Unlock()

	h.flushItems = count
}

func (h *hubRec) SetFlushThreshold (count int) {
	h.flushLock5.Lock()
	defer h.flushLock5.Unlock()

	h.flushThreshold = count
}

func (h *hubRec) cleanup () {
	if h.flushTimer != nil {
		h.flushTimer.Stop()
		h.flushTimer = nil
	}

	h.flush()
	h.stopSignal <- true
}

func (h *hubRec) goSend () {
	for parcel := range h.parcelQueue {
		parcel.Func(parcel.Conn)
		h.senderGroup.Done()
	}
}

func (h *hubRec) queueParcel (cid int, f sendFunc) {
	c := h.conns[cid]
	if c == nil {
		return
	}

	h.senderGroup.Add(1)
	h.parcelQueue <- &parcelRec {c, f}
}

func (h *hubRec) goPickTask () {
	var cid int

	h.parcelQueue = make(chan *parcelRec)
	for i := h.senderCnt; i > 0; i-- {
		go h.goSend()
	}

	for task := range h.taskQueue {
		h.connLock20.RLock()

		switch task.Target {
			case connTarget:
				h.queueParcel(task.Id, task.Func)

			case userTarget:
				for _, cid = range h.userConnIds[task.Id] {
					h.queueParcel(cid, task.Func)
				}

			case globalTarget:
				for cid := range h.conns {
					h.queueParcel(cid, task.Func)
				}

			case roomTarget:
				h.roomLock30.RLock()
				room := h.rooms[task.Id]
				if room != nil {
					for _, uid := range room.UserIds {
						for _, cid = range h.userConnIds[uid] {
							h.queueParcel(cid, task.Func)
						}
					}
				}
				h.roomLock30.RUnlock()
		}

		h.connLock20.RUnlock()
		h.senderGroup.Wait()
	}

	close(h.parcelQueue)
}

func (h *hubRec) Start () {
	if h.isRunning {
		return
	}

	h.flushLock5.Lock()
	defer h.flushLock5.Unlock()

	h.taskQueue = make(chan *taskRec, taskQueueLen)
	h.stopSignal = make(chan bool, 1)

	go h.goPickTask()

	h.isRunning = true

	if h.flushDelay > 0 {
		h.flushTimer = time.AfterFunc(h.flushDelay, h.flush)
	}
}

func (h *hubRec) Stop () {
	if !h.isRunning {
		return
	}

	h.isRunning = false

	close(h.taskQueue)

	if len(h.conns) == 0 {
		h.cleanup()
	}

	<- h.stopSignal
}

func (h *hubRec) Connect (c Conn) error {
	if !h.isRunning {
		return Stopped
	}

	connId := c.Id()
	userId := c.UserId()

	h.connLock20.Lock()
	defer h.connLock20.Unlock()

	if h.conns[connId] != nil {
		return errors.New("connection already registered")
	}

	h.conns[connId] = c
	h.userConnIds[userId] = append(h.userConnIds[userId], connId)

	return nil
}

func (h *hubRec) Disconnect (connId int) {
	h.connLock20.Lock()
	defer h.connLock20.Unlock()

	c := h.conns[connId]
	if c == nil {
		return
	}

	delete(h.conns, connId)

	userId := c.UserId()
	connIds := h.userConnIds[userId]
	lastIndex := len(connIds) - 1
	for i, cid := range connIds {
		if cid != connId {
			continue
		}

		connIds[i] = connIds[lastIndex]
		h.userConnIds[userId] = connIds[:lastIndex]
		break
	}

	if len(h.userConnIds[userId]) > 0 {
		return
	}

	delete(h.userConnIds, userId)

	if !h.isRunning && len(h.conns) == 0 {
		h.cleanup()
	}
}

func (h *hubRec) IsConnected (connId int) bool {
	h.connLock20.RLock()
	defer h.connLock20.RUnlock()
	_, is := h.conns[connId]
	return is
}

func (h *hubRec) Connection (connId int) Conn {
	h.connLock20.RLock()
	defer h.connLock20.RUnlock()
	return h.conns[connId]
}


func (h *hubRec) NewRoom (roomId, lastMessageId int, userIds []int) {
	h.roomLock30.Lock()
	defer h.roomLock30.Unlock()

	if h.rooms[roomId] != nil {
		return
	}

	room := &roomRec {userIds, lastMessageId}
	h.rooms[roomId] = room
}

func (h *hubRec) DeleteRoom (roomId int) {
	h.roomLock30.Lock()
	defer h.roomLock30.Unlock()

	if h.rooms[roomId] == nil {
		return
	}

	delete(h.rooms, roomId)
}

func (h *hubRec) EnterRoom (userId, roomId int) error {
	h.connLock20.RLock()
	h.roomLock30.Lock()
	defer func () {
		h.roomLock30.Unlock()
		h.connLock20.RUnlock()
	}()

	room := h.rooms[roomId]
	if room == nil {
		return errors.New("room not found")
	}

	for _, uid := range room.UserIds {
		if uid == userId {
			return nil
		}
	}

	room.UserIds = append(room.UserIds, userId)
	return nil
}

func (h *hubRec) LeaveRoom (userId, roomId int) {
	h.connLock20.RLock()
	h.roomLock30.Lock()
	defer func () {
		h.roomLock30.Unlock()
		h.connLock20.RUnlock()
	}()

	room := h.rooms[roomId]
	if room == nil {
		return
	}

	uids := room.UserIds
	lastIndex := len(uids) - 1
	for i, uid := range uids {
		if uid != userId {
			continue
		}

		uids[i] = uids[lastIndex]
		room.UserIds = uids[:lastIndex]
	}
}

func (h *hubRec) NewMessage (connId, roomId int, data interface {}) (messageId int, e error) {
	if !h.isRunning {
		return 0, Stopped
	}

	h.flushLock5.Lock()
	h.messageLock10.Lock()
	h.connLock20.RLock()
	h.roomLock30.RLock()
	defer func () {
		h.roomLock30.RUnlock()
		h.connLock20.RUnlock()
		h.messageLock10.Unlock()
		h.flushLock5.Unlock()
	}()

	userId := 0
	if connId != 0 {
		conn := h.conns[connId]
		if conn == nil {
			return 0, errors.New("connection not found")
		}

		userId = conn.UserId()
	}

	room := h.rooms[roomId]
	if room == nil {
		return 0, errors.New("room not found")
	}

	room.LastMessageId++
	entry := &MessageEntry {
		RoomId: roomId,
		MessageId: room.LastMessageId,
		UserId: userId,
		Timestamp: int(time.Now().Unix()),
		Data: data,
	}

	h.messages = append(h.messages, entry)
	h.taskQueue <- &taskRec {roomTarget, roomId, func (c Conn) {
		c.NewMessage(entry)
	}}

	if h.flushThreshold > 0 && len(h.messages) > h.flushThreshold {
		h.flush()
	}

	return entry.MessageId, nil
}

func (h *hubRec) UpdateMessage (roomId, messageId int, data interface {}) error {
	if !h.isRunning {
		return Stopped
	}

	h.flushLock5.Lock()
	defer h.flushLock5.Unlock()

	changed, e := h.storage.Update(roomId, messageId, data)
	if e != nil {
		return e
	}

	if changed {
		return nil
	}

	h.messageLock10.Lock()
	defer h.messageLock10.Unlock()

	for i, entry := range h.messages {
		if entry.RoomId != roomId || entry.MessageId != messageId {
			continue
		}

		h.messages[i].Data = data
		h.taskQueue <- &taskRec {roomTarget, roomId, func (c Conn) {
			c.UpdateMessage(entry)
		}}
		return nil
	}

	return errors.New("message not found")
}

func (h *hubRec) notice (target, id int, data interface {}) error {
	if !h.isRunning {
		return Stopped
	}

	h.taskQueue <- &taskRec {target, id, func (c Conn) {
		c.Notice(data)
	}}

	return nil
}

func (h *hubRec) ConnNotice (connId int, data interface {}) error {
	return h.notice(connTarget, connId, data)
}

func (h *hubRec) UserNotice (userId int, data interface {}) error {
	return h.notice(userTarget, userId, data)
}

func (h *hubRec) RoomNotice (roomId int, data interface {}) error {
	return h.notice(roomTarget, roomId, data)
}

func (h *hubRec) GlobalNotice (data interface {}) error {
	return h.notice(globalTarget, 0, data)
}

func (h *hubRec) Messages (userId, roomId, firstId, count int) (MessageList, error) {
	if count <= 0 {
		count = 10
	}

	h.flushLock5.Lock()
	defer h.flushLock5.Unlock()
	h.roomLock30.RLock()
	defer h.roomLock30.RUnlock()

	room := h.rooms[roomId]
	if room == nil {
		return MessageList {}, errors.New("room not found")
	}

	inRoom := false
	for _, uid := range room.UserIds {
		if uid != userId {
			continue
		}

		inRoom = true
		break
	}

	if !inRoom {
		return MessageList {}, errors.New("user not in this room")
	}

	lastMessageId := room.LastMessageId
	if firstId < 0 {
		firstId += lastMessageId + 1
	}
	if firstId < 1 {
		firstId = 1
	}

	messages, e := h.storage.List(roomId, firstId, count)
	if e != nil {
		return messages, e
	}

	gotCnt := len(messages)
	if gotCnt >= count || (gotCnt > 0 && messages[gotCnt - 1].MessageId >= lastMessageId) {
		return messages, nil
	}

	h.messageLock10.RLock()
	defer h.messageLock10.RUnlock()

	for _, message := range h.messages {
		if message.RoomId != roomId {
			continue
		}

		messages = append(messages, message)
		gotCnt++
		if gotCnt >= count {
			break
		}
	}

	return messages, nil
}

func (h *hubRec) UserRoomIds (userId int) []int {
	h.connLock20.RLock()
	h.roomLock30.RLock()
	defer func () {
		h.roomLock30.RUnlock()
		h.connLock20.RUnlock()
	}()

	roomIds := make([]int, 0, 4)
	for rid, room := range h.rooms {
		for _, uid := range room.UserIds {
			if uid != userId {
				continue
			}

			roomIds = append(roomIds, rid)
			break
		}
	}

	return roomIds
}

func (h *hubRec) IsInRoom (userId, roomId int) bool {
	h.roomLock30.RLock()
	defer h.roomLock30.RUnlock()

	room := h.rooms[roomId]
	if room == nil {
		return false
	}

	for _, uid := range room.UserIds {
		if uid == userId {
			return true
		}
	}

	return false
}


func (h *hubRec) OnlineUserIds () []int {
	if !h.isRunning {
		return make([]int, 0)
	}

	h.connLock20.RLock()
	defer h.connLock20.RUnlock()

	userIds := make([]int, 0)
	for uid, conns := range h.userConnIds {
		if len(conns) == 0 {
			continue
		}

		userIds = append(userIds, uid)
	}

	return userIds
}

func (h *hubRec) RoomUserIds (roomId int) []int {
	h.roomLock30.RLock()
	defer h.roomLock30.RUnlock()

	room := h.rooms[roomId]
	if room == nil {
		return []int {}
	}

	return room.UserIds
}

func (h *hubRec) UserConnIds (userId int) []int {
	if !h.isRunning {
		return make([]int, 0)
	}

	h.connLock20.RLock()
	defer h.connLock20.RUnlock()

	return h.userConnIds[userId]
}

func (h *hubRec) UserIsConnected (userId int) bool {
	if !h.isRunning {
		return false
	}

	h.connLock20.RLock()
	defer h.connLock20.RUnlock()

	return (len(h.userConnIds[userId]) > 0)
}
