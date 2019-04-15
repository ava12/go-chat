package hub

import (
	"testing"
	"log"
	"fmt"
	"math/rand"
	"time"
)

func init () {
	rand.Seed(time.Now().UnixNano())
}


var reports chan string

func goLogAnomalies () {
	anomalies := make(map[string]bool)

	for report := range reports {
		if report == "" {
			break
		}

		if anomalies[report] {
			continue
		}

		anomalies[report] = true
		log.Printf("anomaly: %s\n", report)
	}

	log.Printf("total anomalies: %d\n", len(anomalies))
}

// новое сообщение с номером меньше номера предыдущего
func reportMessageOrder (roomId, messageId, lastMessageId int) {
	reports <- fmt.Sprintf("wrong message order: r%dm%d > r%[1]dm%[3]d", roomId, messageId, lastMessageId)
}

// пропуск в нумерации сообщений
func reportMissingMessage (roomId, messageId, lastMessageId int) {
	reports <- fmt.Sprintf("missing message: r%dm%[3]d ... r%[1]dm%[2]d", roomId, messageId, lastMessageId)
}

// соообщение, пришедшее к пользователю, отсутствующему в комнате
func reportMisplacedMessage (roomId, userId, messageId, recipientId int) {
	reports <- fmt.Sprintf("message to stranger: r%du%dm%d -> u%d", roomId, userId, messageId, recipientId)
}

// сообщение от пользователя, отсутствующего в комнате
func reportStrangerMessage (roomId, userId, messageId int) {
	reports <- fmt.Sprintf("message from stranger: r%du%dm%d", roomId, userId, messageId)
}

// повторное уведомление о входе того же пользователя в ту же комнату
func reportDoubleEnter (roomId, userId int) {
	reports <- fmt.Sprintf("entering room again: r%du%d", roomId, userId)
}

// уведомление о выходе пользователя из комнаты, в которой он не находился
func reportStrangerLeave (roomId, userId int) {
	reports <- fmt.Sprintf("leaving wrong room: r%du%d", roomId, userId)
}

// уведомление для отключенного соединения
func reportNecromancy (connId int) {
	reports <- fmt.Sprintf("addressing disconnected: c%d", connId)
}

// повторное уведомление (подключение, отключение)
func reportDoubleNotice (connId int, notice string) {
	reports <- fmt.Sprintf("double notice for c%d: %s", connId, notice)
}


const (
	connectUserNotice = iota
	disconnectUserNotice
	enterRoomNotice
	leaveRoomNotice
)

type noticeRec struct {
	Type int
	ConnId, UserId, RoomId int
}

const (
	userCnt = 12
	connsPerUser = 4
	totalConns = userCnt * connsPerUser
	userConnMask = (1 << connsPerUser) - 1
	roomCnt = 5
	userRoomMask = (1 << roomCnt) - 1
)

type testConn struct {
	id, userId, users int
	connected bool
	hub Hub
	userRooms [userCnt]int
	lastMessageIds [roomCnt]int
}

func newTestConn (h Hub, id, userId int) *testConn {
	return &testConn {id: id, userId: userId, hub: h}
}

func (c *testConn) toggle () {
	if c.connected {
		c.hub.Disconnect(c.id)
		connIds := c.hub.UserConnIds(c.userId)
		if len(connIds) == 0 {
			c.hub.GlobalNotice(&noticeRec {disconnectUserNotice, c.id, c.userId, 0})
		}
	} else {
		connIds := c.hub.UserConnIds(c.userId)
		c.hub.Connect(c)
		if len(connIds) == 0 {
			c.hub.GlobalNotice(&noticeRec {connectUserNotice, c.id, c.userId, 0})
		}
	}
	c.connected = !c.connected
}

func (c *testConn) move () int {
	roomId := rand.Intn(roomCnt)
	mask := 1 << uint(roomId)
	if c.userRooms[c.userId] & mask != 0 {
		c.hub.LeaveRoom(c.userId, roomId)
		c.hub.RoomNotice(roomId, &noticeRec {leaveRoomNotice, c.id, c.userId, roomId})
	} else {
		c.hub.EnterRoom(c.userId, roomId)
		c.hub.RoomNotice(roomId, &noticeRec {enterRoomNotice, c.id, c.userId, roomId})
	}

	c.userRooms[c.userId] ^= mask
	c.lastMessageIds[roomId] = 0
	return roomId
}

func (c *testConn) speak () {
	if !c.connected {
		c.toggle() // одно
//		return // из двух
	}

	var roomId int

	mask := c.userRooms[c.userId]
	if mask == 0 {
		roomId = c.move()
	} else {
		ids := make([]int, 0, roomCnt)
		for i := 0; i < roomCnt; i++ {
			if mask & 1 != 0 {
				ids = append(ids, i)
			}
			mask >>= 1
		}
		roomId = ids[rand.Intn(len(ids))]
	}

	c.hub.NewMessage(c.id, roomId, 123)
}

func (c *testConn) Id () int {
	return c.id
}

func (c *testConn) UserId () int {
	return c.userId
}

func (c *testConn) NewMessage (m *MessageEntry) {
	if !c.connected {
		reportNecromancy(c.id)
		return
	}

	roomMask := 1 << uint(m.RoomId)

	if c.userRooms[c.userId] & roomMask == 0 {
		reportMisplacedMessage(m.RoomId, m.UserId, m.MessageId, c.userId)
		return
	}

	lastMessageId := c.lastMessageIds[m.RoomId]

	if m.MessageId < lastMessageId {
		reportMessageOrder(m.RoomId, m.MessageId, lastMessageId)
	} else {
		if lastMessageId > 0 && (lastMessageId + 1) != m.MessageId {
			reportMissingMessage(m.RoomId, m.MessageId, lastMessageId)
		}
		c.lastMessageIds[m.RoomId] = m.MessageId
	}

	if c.userRooms[m.UserId] & roomMask == 0 {
		reportStrangerMessage(m.RoomId, m.UserId, m.MessageId)
	}
}

func (c *testConn) UpdateMessage (m *MessageEntry) {}

func (c *testConn) Notice (data interface {}) {
	if !c.connected {
		reportNecromancy(c.id)
		return
	}

	notice := data.(*noticeRec)
	if notice.ConnId == c.id {
		return
	}

	switch notice.Type {
		case connectUserNotice:
			mask := 1 << uint(notice.UserId)
			if c.users & mask != 0 {
				reportDoubleNotice(c.id, fmt.Sprintf("connect u%d", notice.UserId))
			} else {
				c.users |= mask
			}

		case disconnectUserNotice:
			mask := 1 << uint(notice.UserId)
			if c.users & mask == 0 {
				reportDoubleNotice(c.id, fmt.Sprintf("disconnect u%d", notice.UserId))
			} else {
				c.users &^= mask
			}

		case enterRoomNotice:
			mask := 1 << uint(notice.RoomId)
			if c.userRooms[notice.UserId] & mask != 0 {
				reportDoubleEnter(notice.RoomId, notice.UserId)
			} else {
				c.userRooms[notice.UserId] &= mask
			}

		case leaveRoomNotice:
			mask := 1 << uint(notice.RoomId)
			if c.userRooms[notice.UserId] & mask == 0 {
				reportStrangerLeave(notice.RoomId, notice.UserId)
			} else {
				c.userRooms[notice.UserId] &^= mask
			}
	}
}


type noStorage bool

func (noStorage) Save (m MessageList) error {
	return nil
}

func (noStorage) List (roomId, firstId, count int) (MessageList, error) {
	return MessageList {}, nil
}

func (noStorage) Update (roomId, messageId int, data interface {}) (bool, error) {
	return true, nil
}


const (
	toggleEvent = iota
	moveEvent
	speakEvent
)

const eventTypeCnt = 3

// относительные частоты генерируемых событий
// при увеличении доли (под/от)ключений появляются аномалии двойных уведомлений
const (
	// (под/от)ключение к/от хаба
	toggleFreq = 1

	// вход/выход из случайно выбранной комнаты; не зависит от статуса подключения
	moveFreq = 4

	// сообщение в случайно выбранной доступной комнате;
	// если доступных нет, то выполняется вход в случайную комнату
	speakFreq = 20
)

const (
	maxEventsPerTick = 4
	tickPeriod = 1 * time.Millisecond
	totalTicks = 1000
)

func eventFreqs () []int {
	result := make([]int, toggleFreq + moveFreq + speakFreq)
	i := 0
	for event, freq := range []int {toggleFreq, moveFreq, speakFreq} {
		for freq > 0 {
			result[i] = event
			freq--
			i++
		}
	}

	return result
}

func goPickEvent (c *testConn, ch <-chan int) {
	for event := range ch {
		switch event {
			case speakEvent:
				c.speak()
			case moveEvent:
				c.move()
			case toggleEvent:
				c.toggle()
		}
	}
}

func TestAnomalies (t *testing.T) {
	hub := New(noStorage(true))

	reports = make(chan string, 10)
	go goLogAnomalies()

	conns := make([]*testConn, totalConns)
	chans := make([]chan int, totalConns)
	for i := range conns {
		conns[i] = newTestConn(hub, i, int(i / connsPerUser))
		chans[i] = make(chan int, maxEventsPerTick)
		go goPickEvent(conns[i], chans[i])
	}

	freqs := eventFreqs()
	totalEvents := len(freqs)

	var events [eventTypeCnt]int

	hub.Start()

	for i := totalTicks; i > 0; i-- {
		for j := rand.Intn(maxEventsPerTick + 1); j > 0; j-- {
			event := freqs[rand.Intn(totalEvents)]
			connId := rand.Intn(totalConns)
			chans[connId] <- event
			events[event]++
		}
		time.Sleep(tickPeriod)
	}

	for i, ch := range chans {
		close(ch)
		hub.Disconnect(i)
	}
	reports <- ""
	time.Sleep(1 * time.Second)
	close(reports)

	hub.Stop()
	log.Printf("events generated: %v", events)
}
