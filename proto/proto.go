package proto

import (
	"github.com/ava12/go-chat/hub"
	"encoding/json"
	"sync"
	"strings"
	"errors"
	"log"
	"fmt"
)

type PermFlags = int

const (
	listRoomsPerm = 1 << iota
	createRoomPerm
	allGlobalPerms = listRoomsPerm | createRoomPerm
)

const (
	readPerm = 1 << iota
	writePerm
	allRoomPerms = readPerm | writePerm
)

type AccessController interface {
	GlobalPerms (userId int) PermFlags
	RoomPerms (userId, roomId int) PermFlags
	HasGlobalPerm (userId, perm int) bool
	HasRoomPerm (userId, roomId, perm int) bool
	NewRoom (userId, roomId int)
}

type accessRec struct {}

func NewAccessController () AccessController {
	return &accessRec {}
}

func (ar *accessRec) GlobalPerms (userId int) PermFlags {
	return allGlobalPerms
}

func (ar *accessRec) RoomPerms (userId, roomId int) PermFlags {
	return allRoomPerms
}

func (ar *accessRec) HasGlobalPerm (userId, perm int) bool {
	return (perm & allGlobalPerms != 0)
}

func (ar *accessRec) HasRoomPerm (userId, roomId, perm int) bool {
	return (perm & allRoomPerms != 0)
}

func (ar *accessRec) NewRoom (userId, roomId int) {}


type request struct {
	Request string `json:"request"`
	Body json.RawMessage `json:"body"`
}

const (
	messageReq = "message"
	whoamiReq = "whoami"
	listRoomsReq = "list-rooms"
	inRoomsReq = "in-rooms"
	enterReq = "enter"
	leaveReq = "leave"
	newRoomReq = "new-room"
	listUsersReq = "list-users"
	listMessagesReq = "list-messages"
)

type response struct {
	Response string `json:"response"`
	Body interface {} `json:"body"`
}

const (
	errorResp = "error"
	messageResp = "message"
	whoamiResp = "whoami"
	updateMessageResp = "update-message"
	listRoomsResp = "list-rooms"
	inRoomsResp = "in-rooms"
	enterResp = "enter"
	leaveResp = "leave"
	newRoomResp = "new-room"
	listUsersResp = "list-users"
	listMessagesResp = "list-messages"
)

type errorResponse struct {
	Message string `json:"message"`
}

type messageRequest struct {
	RoomId int `json:"roomId"`
	MessageType int `json:"messageType"`
	Data json.RawMessage `json:"data"`
}

const (
	textMessageType = iota + 1
)

type textMessageData struct {
	Text string `json:"text"`
}

type messageResponse MessageEntry
type hubMessageData struct {
	MessageType int `json:"messageType"`
	Data interface {} `json:"data"`
}

type whoamiResponse struct {
	User UserEntry `json:"user"`
	Perm PermFlags  `json:"perm"`
}

type listRoomsResponse struct {
	Rooms []RoomEntry `json:"rooms"`
}

type inRoomsResponse = listRoomsResponse

type newRoomRequest struct {
	Name string `json:"name"`
}

type newRoomResponse struct {
	Id int `json:"id"`
	Name string `json:"name"`
}

type enterRequest struct {
	RoomId int `json:"roomId"`
}

type enterResponse struct {
	RoomId int `json:"roomId"`
	User UserEntry `json:"user"`
	Perm PermFlags  `json:"perm"`
}

type leaveRequest struct {
	RoomId int `json:"roomId"`
}

type leaveResponse struct {
	RoomId int `json:"roomId"`
	UserId int `json:"userId"`
}

type listUsersRequest struct {
	RoomId int `json:"roomId"`
}

type listUsersResponse struct {
	RoomId int `json:"roomId"`
	Users []UserEntry `json:"users"`
}

type listMessagesRequest struct {
	RoomId int `json:"roomId"`
	FirstMessageId int `json:"firstMessageId"`
	MessageCnt int `json:"messageCnt"`
}

type listMessagesResponse struct {
	RoomId int `json:"roomId"`
	FirstMessageId int `json:"firstMessageId"`
	Messages MessageList `json:"messages"`
}

type MessageEntry struct {
	RoomId int `json:"roomId"`
	MessageId int `json:"messageId"`
	UserId int `json:"userId"`
	Timestamp int `json:"timestamp"`
	Data interface {} `json:"data"`
}

type MessageList []*MessageEntry

type Conn interface {
	Id () int
	UserId () int
	Send (m []byte)
}

type hubConnRec struct {
	c Conn
}

func (c *hubConnRec) Id () int {
	return c.c.Id()
}

func (c *hubConnRec) UserId () int {
	return c.c.UserId()
}

func (c *hubConnRec) send (response interface {}) {
	defer func () {
		e := recover()
		if e != nil {
			log.Println(e)
		}
	}()

	data, e := json.Marshal(response)
	if e != nil {
		log.Println(e.Error())
	} else {
		c.c.Send(data)
	}
}

func (c *hubConnRec) NewMessage (m *hub.MessageEntry) {
	c.send(response {messageResp, m})
}

func (c *hubConnRec) UpdateMessage (m *hub.MessageEntry) {
	c.send(response {updateMessageResp, m})
}

func (c *hubConnRec) Notice (data interface {}) {
	c.send(data)
}


type RoomEntry struct {
	Id int `json:"id"`
	Name string `json:"name"`
}

type RoomRegistry interface {
	ListRooms () []RoomEntry
	CreateRoom (name string) (id int, e error)
	Room (id int) (RoomEntry, bool)
}

type memRegistryRec struct {
	lock sync.RWMutex
	rooms map[int]*RoomEntry
	lastId int
}

func NewRoomRegistry () RoomRegistry {
	return &memRegistryRec {rooms: make(map[int]*RoomEntry)}
}

func (mrr *memRegistryRec) ListRooms () []RoomEntry {
	mrr.lock.RLock()
	defer mrr.lock.RUnlock()

	result := make([]RoomEntry, 0, len(mrr.rooms))
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
			return 0, errors.New("room already exists")
		}
	}

	mrr.lastId++
	mrr.rooms[mrr.lastId] = &RoomEntry {mrr.lastId, name}
	return mrr.lastId, nil
}

func (mrr *memRegistryRec) Room (id int) (RoomEntry, bool) {
	mrr.lock.RLock()
	defer mrr.lock.RUnlock()

	entry := mrr.rooms[id]
	if entry != nil {
		return *entry, true
	} else {
		return RoomEntry {}, false
	}
}


type UserEntry struct {
	Id int `json:"id"`
	Name string `json:"name"`
}

type UserRegistry interface {
	User (id int) (UserEntry, bool)
}


type Proto interface {
	Connect (c Conn)
	Disconnect (connId int)
	TakeRequest (c Conn, r []byte)
}

type requestHandler func (Conn, []byte)

type protoRec struct {
	hub hub.Hub
	users UserRegistry
	rooms RoomRegistry
	access AccessController
	handlers map[string]requestHandler
}

func New (hub hub.Hub, users UserRegistry, rooms RoomRegistry, access AccessController) Proto {
	if hub == nil {
		panic("no chat hub")
	}

	if users == nil {
		panic("no user registry")
	}

	if rooms == nil {
		panic("no room registry")
	}

	if access == nil {
		panic("no access controller")
	}

	p := &protoRec {hub: hub, users: users, rooms: rooms, access: access}

	hs := make(map[string]requestHandler)

	hs[messageReq] = p.newMessage
	hs[whoamiReq] = p.whoami
	hs[listRoomsReq] = p.listRooms
	hs[inRoomsReq] = p.inRooms
	hs[enterReq] = p.enterRoom
	hs[leaveReq] = p.leaveRoom
	hs[listUsersReq] = p.listUsers
	hs[listMessagesReq] = p.listMessages

	p.handlers = hs
	return p
}

func (p *protoRec) Connect (c Conn) {
	p.hub.Connect(&hubConnRec {c})
}

func (p *protoRec) Disconnect (connId int) {
	hc := p.hub.Connection(connId)
	if hc == nil {
		return
	}

	uid := hc.Id()
	p.hub.Disconnect(connId)
	if p.hub.UserIsConnected(uid) {
		return
	}

	rids := p.hub.UserRoomIds(uid)
	for _, rid := range rids {
		p.hub.LeaveRoom(uid, rid)
		resp := &response {leaveResp, leaveResponse {rid, uid}}
		p.hub.RoomNotice(rid, resp)
	}
}

func (p *protoRec) TakeRequest (c Conn, r []byte) {
	cid := c.Id()
	if !p.hub.IsConnected(cid) {
		return
	}

	req := &request {}
	e := json.Unmarshal(r, req)
	if e != nil {
		log.Println(e)
		return
	}

	handler := p.handlers[req.Request]
	if handler != nil {
		handler(c, req.Body)
	} else {
		log.Printf("u%dc%d: unknown request type: %s", c.UserId(), cid, req.Request)
	}
}

func (p *protoRec) respondError (c Conn, m string) {
	cid := c.Id()
	uid := c.UserId()
	response := &response {errorResp, errorResponse {m}}
	log.Printf("u%dc%d: %s", uid, cid, m)
	p.hub.ConnNotice(cid, response)
}

func (p *protoRec) decodeBody (c Conn, body []byte, v interface {}) bool {
	e := json.Unmarshal(body, v)
	if e == nil {
		return true
	}

	p.respondError(c, e.Error())
	return false
}

func (p *protoRec) whoami (c Conn, body []byte) {
	uid := c.UserId()
	user, _ := p.users.User(uid)
	perm := p.access.GlobalPerms(uid)
	resp := &response {whoamiResp, whoamiResponse {user, perm}}
	p.hub.ConnNotice(c.Id(), resp)
}

func (p *protoRec) listRooms (c Conn, body []byte) {
	if !p.access.HasGlobalPerm(c.UserId(), listRoomsPerm) {
		p.respondError(c, "you cannot list rooms")
		return
	}

	resp := &response {listRoomsResp, listRoomsResponse {p.rooms.ListRooms()}}
	p.hub.ConnNotice(c.Id(), resp)
}

func (p *protoRec) inRooms (c Conn, body []byte) {
	uid := c.UserId()
	rids := p.hub.UserRoomIds(uid)
	result := make([]RoomEntry, 0, len(rids))
	for _, rid := range rids {
		room, found := p.rooms.Room(rid)
		if found {
			result = append(result, room)
		}
	}

	resp := &response {inRoomsResp, inRoomsResponse {result}}
	p.hub.ConnNotice(c.Id(), resp)
}

func (p *protoRec) createRoom (c Conn, body []byte) {
	b := &newRoomRequest {}
	if !p.decodeBody(c, body, b) {
		return
	}

	uid := c.UserId()
	if !p.access.HasGlobalPerm(uid, createRoomPerm) {
		p.respondError(c, "you cannot create a room")
		return
	}

	name := strings.TrimSpace(b.Name)
	if name == "" {
		p.respondError(c, "empty room name")
		return
	}

	rid, e := p.rooms.CreateRoom(name)
	if e != nil {
		p.respondError(c, e.Error())
		return
	}

	p.access.NewRoom(uid, rid)
	resp := &response {newRoomResp, newRoomResponse {rid, name}}
	p.hub.GlobalNotice(resp)
}

func (p *protoRec) enterRoom (c Conn, body []byte) {
	b := &enterRequest {}
	if !p.decodeBody(c, body, b) {
		return
	}

	if !p.access.HasRoomPerm(c.UserId(), b.RoomId, readPerm) {
		p.respondError(c, "you cannot enter this room")
		return
	}

	uid := c.UserId()
	e := p.hub.EnterRoom(uid, b.RoomId)
	if e != nil {
		p.respondError(c, e.Error())
		return
	}

	user, _ := p.users.User(uid)
	perm := p.access.RoomPerms(uid, b.RoomId)
	resp := &response {enterResp, enterResponse {b.RoomId, user, perm}}
	p.hub.RoomNotice(b.RoomId, resp)
}

func (p *protoRec) leaveRoom (c Conn, body []byte) {
	b := &leaveRequest {}
	if !p.decodeBody(c, body, b) {
		return
	}

	uid := c.UserId()
	p.hub.LeaveRoom(uid, b.RoomId)
	resp := &response {leaveResp, leaveResponse {b.RoomId, uid}}
	p.hub.ConnNotice(c.Id(), resp)
	p.hub.RoomNotice(b.RoomId, resp)
}

func (p *protoRec) listUsers (c Conn, body []byte) {
	b := &listUsersRequest {}
	if !p.decodeBody(c, body, b) {
		return
	}

	uid := c.UserId()
	if !p.hub.IsInRoom(uid, b.RoomId) {
		p.respondError(c, "user not in room")
		return
	}

	userIds := p.hub.RoomUserIds(b.RoomId)
	result := make([]UserEntry, 0, len(userIds))
	for _, id := range userIds {
		entry, present := p.users.User(id)
		if present {
			result = append(result, entry)
		}
	}

	resp := &response {listUsersResp, listUsersResponse {b.RoomId, result}}
	p.hub.ConnNotice(c.Id(), resp)
}

func (p *protoRec) listMessages (c Conn, body []byte) {
	b := &listMessagesRequest {}
	if !p.decodeBody(c, body, b) {
		return
	}

	uid := c.UserId()
	messages, e := p.hub.Messages(uid, b.RoomId, b.FirstMessageId, b.MessageCnt)
	if e != nil {
		p.respondError(c, e.Error())
		return
	}

	result := make(MessageList, 0, len(messages))
	for _, m := range messages {
		result = append(result, (*MessageEntry)(m))
	}

	resp := &response {listMessagesResp, listMessagesResponse {b.RoomId, b.FirstMessageId, result}}
	p.hub.ConnNotice(c.Id(), resp)
}

func (p *protoRec) newMessage (c Conn, body []byte) {
	b := &messageRequest {}
	if !p.decodeBody(c, body, b) {
		return
	}

	if !p.access.HasRoomPerm(c.UserId(), b.RoomId, writePerm) {
		p.respondError(c, "you cannot post messages in this room")
		return
	}

	switch b.MessageType {
		case textMessageType:
			p.newTextMessage(c, b.RoomId, b.Data)

		default:
			p.respondError(c, fmt.Sprintf("unknown message type: %d", b.MessageType))
	}
}

func (p *protoRec) newTextMessage (c Conn, roomId int, data []byte) {
	d := &textMessageData {}
	if !p.decodeBody(c, data, d) {
		return
	}

	d.Text = strings.TrimSpace(d.Text)
	if d.Text == "" {
		p.respondError(c, "empty message text")
		return
	}

	hubData := &hubMessageData {textMessageType, d}
	_, e := p.hub.NewMessage(c.Id(), roomId, hubData)
	if e != nil {
		p.respondError(c, e.Error())
	}
}
