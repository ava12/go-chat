package simple

import (
	"github.com/ava12/go-chat/hub"
	"github.com/ava12/go-chat/conn"
	"github.com/ava12/go-chat/user"
	"github.com/ava12/go-chat/room"
	"github.com/ava12/go-chat/access"
	"encoding/json"
	"strings"
	"log"
	"fmt"
)


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
	userInfoReq = "user-info"
	roomInfoReq = "room-info"
)

type response struct {
	Response string `json:"response"`
	Body interface {} `json:"body"`
}

const (
	errorResp = "error"
	messageResp = "message"
	whoamiResp = "whoami"
	listRoomsResp = "list-rooms"
	inRoomsResp = "in-rooms"
	enterResp = "enter"
	leaveResp = "leave"
	newRoomResp = "new-room"
	listUsersResp = "list-users"
	listMessagesResp = "list-messages"
	userInfoResp = "user-info"
	roomInfoResp = "room-info"
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
	User interface{} `json:"user"`
	Perm access.PermFlags  `json:"perm"`
}

type listRoomsResponse struct {
	Rooms []RoomPermEntry `json:"rooms"`
}

type inRoomsResponse listRoomsResponse

type newRoomRequest struct {
	Name string `json:"name"`
}

type newRoomResponse RoomPermEntry

type enterRequest struct {
	RoomId int `json:"roomId"`
}

type enterResponse struct {
	RoomId int `json:"roomId"`
	User interface{} `json:"user"`
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
	Users []interface{} `json:"users"`
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

type userInfoRequest struct {
	UserId int `json:"userId"`
}

type userInfoResponse interface{}

type roomInfoRequest struct {
	RoomId int `json:"roomId"`
}

type roomInfoResponse RoomPermEntry


type MessageEntry struct {
	RoomId int `json:"roomId"`
	MessageId int `json:"messageId"`
	UserId int `json:"userId"`
	Timestamp int `json:"timestamp"`
	Data interface {} `json:"data"`
}

type MessageList []*MessageEntry


type hubConnRec struct {
	c conn.Conn
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
	c.send(response {messageResp, MessageEntry(*m)})
}

func (c *hubConnRec) UpdateMessage (m *hub.MessageEntry) {
}

func (c *hubConnRec) Notice (data interface {}) {
	c.send(data)
}

func (c *hubConnRec) Close () {
	c.c.Close()
}


type RoomPermEntry struct {
	Id int `json:"id"`
	Name string `json:"name"`
	Perm int `json:"perm"`
}


type requestHandler func (conn.Conn, []byte)


type Proto struct {
	hub *hub.Hub
	users user.Registry
	rooms room.Registry
	access access.Controller
	handlers map[string]requestHandler
}

func New (hub *hub.Hub, users user.Registry, rooms room.Registry, access access.Controller) *Proto {
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

	p := &Proto {hub: hub, users: users, rooms: rooms, access: access}

	hs := make(map[string]requestHandler)

	hs[enterReq] = p.enterRoom
	hs[inRoomsReq] = p.inRooms
	hs[listRoomsReq] = p.listRooms
	hs[leaveReq] = p.leaveRoom
	hs[listUsersReq] = p.listUsers
	hs[listMessagesReq] = p.listMessages
	hs[messageReq] = p.newMessage
	hs[newRoomReq] = p.createRoom
	hs[roomInfoReq] = p.roomInfo
	hs[userInfoReq] = p.userInfo
	hs[whoamiReq] = p.whoami

	p.handlers = hs
	return p
}

func (p *Proto) Connect (c conn.Conn) {
	p.hub.Connect(&hubConnRec {c})
}

func (p *Proto) Disconnect (connId int) {
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

func (p *Proto) Stop () {

}

func (p *Proto) TakeRequest (c conn.Conn, r []byte) {
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

func (p *Proto) respondError (c conn.Conn, m string, param ... interface {}) {
	if len(param) > 0 {
		m = fmt.Sprintf(m, param...)
	}
	cid := c.Id()
	uid := c.UserId()
	response := &response {errorResp, errorResponse {m}}
	log.Printf("u%dc%d: %s", uid, cid, m)
	p.hub.ConnNotice(cid, response)
}

func (p *Proto) decodeBody (c conn.Conn, body []byte, v interface {}) bool {
	e := json.Unmarshal(body, v)
	if e == nil {
		return true
	}

	p.respondError(c, e.Error())
	return false
}

func (p *Proto) whoami (c conn.Conn, body []byte) {
	uid := c.UserId()
	user, _ := p.users.User(uid)
	perm := p.access.GlobalPerms(uid)
	resp := &response {whoamiResp, whoamiResponse {user, perm}}
	p.hub.ConnNotice(c.Id(), resp)
}

func (p *Proto) listRooms (c conn.Conn, body []byte) {
	uid := c.UserId()
	if !p.access.HasGlobalPerm(uid, access.ListRoomsPerm) {
		p.respondError(c, "you cannot list rooms")
		return
	}

	rooms := p.rooms.ListRooms()
	roomPerms := make([]RoomPermEntry, 0, len(rooms))
	for _, room := range rooms {
		perm := p.access.RoomPerms(uid, room.Id)
		if perm != 0 {
			roomPerms = append(roomPerms, RoomPermEntry {room.Id, room.Name, perm})
		}
	}
	resp := &response {listRoomsResp, listRoomsResponse {roomPerms}}
	p.hub.ConnNotice(c.Id(), resp)
}

func (p *Proto) inRooms (c conn.Conn, body []byte) {
	uid := c.UserId()
	rids := p.hub.UserRoomIds(uid)
	result := make([]RoomPermEntry, 0, len(rids))
	for _, rid := range rids {
		room, found := p.rooms.Room(rid)
		if found {
			perm := p.access.RoomPerms(uid, room.Id)
			result = append(result, RoomPermEntry {room.Id, room.Name, perm})
		}
	}

	resp := &response {inRoomsResp, inRoomsResponse {result}}
	p.hub.ConnNotice(c.Id(), resp)
}

func (p *Proto) createRoom (c conn.Conn, body []byte) {
	b := &newRoomRequest {}
	if !p.decodeBody(c, body, b) {
		return
	}

	uid := c.UserId()
	if !p.access.HasGlobalPerm(uid, access.CreateRoomPerm) {
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
	perm := p.access.RoomPerms(uid, rid)
	p.hub.NewRoom(rid, 0, []int {})
	resp := &response {newRoomResp, newRoomResponse {rid, name, perm}}
	p.hub.GlobalNotice(resp)
}

func (p *Proto) enterRoom (c conn.Conn, body []byte) {
	b := &enterRequest {}
	if !p.decodeBody(c, body, b) {
		return
	}

	if !p.access.HasRoomPerm(c.UserId(), b.RoomId, access.ReadPerm) {
		p.respondError(c, "you cannot enter room #%d", b.RoomId)
		return
	}

	uid := c.UserId()
	e := p.hub.EnterRoom(uid, b.RoomId)
	if e != nil {
		p.respondError(c, e.Error())
		return
	}

	user, _ := p.users.User(uid)
	p.hub.EnterRoom(uid, b.RoomId)
	resp := &response {enterResp, enterResponse {b.RoomId, user}}
	p.hub.RoomNotice(b.RoomId, resp)
}

func (p *Proto) leaveRoom (c conn.Conn, body []byte) {
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

func (p *Proto) listUsers (c conn.Conn, body []byte) {
	b := &listUsersRequest {}
	if !p.decodeBody(c, body, b) {
		return
	}

	uid := c.UserId()
	if !p.hub.IsInRoom(uid, b.RoomId) {
		p.respondError(c, "you are not in room #%d", b.RoomId)
		return
	}

	userIds := p.hub.RoomUserIds(b.RoomId)
	result := make([]interface{}, 0, len(userIds))
	for _, id := range userIds {
		entry, present := p.users.User(id)
		if present {
			result = append(result, entry)
		}
	}

	resp := &response {listUsersResp, listUsersResponse {b.RoomId, result}}
	p.hub.ConnNotice(c.Id(), resp)
}

func (p *Proto) listMessages (c conn.Conn, body []byte) {
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

func (p *Proto) userInfo (c conn.Conn, body []byte) {
	b := &userInfoRequest {}
	if !p.decodeBody(c, body, b) {
		return
	}

	result, has := p.users.User(b.UserId)
	if !has {
		p.respondError(c, "user #%d not found", b.UserId)
		return
	}

	resp := &response {userInfoResp, userInfoResponse(result)}
	p.hub.ConnNotice(c.Id(), resp)
}

func (p *Proto) roomInfo (c conn.Conn, body []byte) {
	b := &roomInfoRequest {}
	if !p.decodeBody(c, body, b) {
		return
	}

	uid := c.UserId()
	perm := p.access.RoomPerms(uid, b.RoomId)
	if perm == 0 {
		p.respondError(c, "room #%d not found", b.RoomId)
		return
	}

	room, has := p.rooms.Room(b.RoomId)
	if !has {
		p.respondError(c, "room #%d not found", b.RoomId)
		return
	}

	resp := &response {roomInfoResp, roomInfoResponse {room.Id, room.Name, perm}}
	p.hub.ConnNotice(c.Id(), resp)
}

func (p *Proto) newMessage (c conn.Conn, body []byte) {
	b := &messageRequest {}
	if !p.decodeBody(c, body, b) {
		return
	}

	if !p.access.HasRoomPerm(c.UserId(), b.RoomId, access.WritePerm) {
		p.respondError(c, "you cannot post messages in room #%d", b.RoomId)
		return
	}

	switch b.MessageType {
		case textMessageType:
			p.newTextMessage(c, b.RoomId, b.Data)

		default:
			p.respondError(c, fmt.Sprintf("unknown message type: %d", b.MessageType))
	}
}

func (p *Proto) newTextMessage (c conn.Conn, roomId int, data []byte) {
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
