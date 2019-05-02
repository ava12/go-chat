package ws

import (
	"net/http"
	"sync"
	"sync/atomic"
	"errors"
	"log"
	"time"
	"github.com/gorilla/websocket"
	"github.com/ava12/go-chat/proto"
)

const (
	maxMessageSize = 65535
)

type Conn interface {
	proto.Conn
	Close ()
}

type connRec struct {
	c *websocket.Conn
	id, userId int
	alive bool
}

func newConn (c *websocket.Conn, id, userId int) *connRec {
	return &connRec {c, id, userId, true}
}

func (c *connRec) Id () int {
	return c.id
}

func (c *connRec) UserId () int {
	return c.userId
}

func (c *connRec) Send (m []byte) {
	if !c.alive {
		return
	}

	e := c.c.WriteMessage(websocket.TextMessage, m)
	if e != nil {
		log.Println(e)
		c.Close()
	}
}

func (c *connRec) Close () {
	c.alive = false
	c.c.Close()
}


type Counter interface {
	Next () int
}

type counterRec struct {
	last int32
}

func NewCounter () Counter {
	return &counterRec {}
}

func (c *counterRec) Next () int {
	return int(atomic.AddInt32(&c.last, 1))
}

var upgrader = websocket.Upgrader {}

type Registry struct {
	lock sync.Mutex
	conns map[int]*connRec
	proto proto.Proto
	ids Counter
	running bool
}

func NewRegistry (p proto.Proto, ids Counter) *Registry {
	if p == nil {
		panic("no chat protocol")
	}

	if ids == nil {
		panic("no connection id generator")
	}

	return &Registry {conns: make(map[int]*connRec), proto: p, ids: ids, running: true}
}

func (reg *Registry) Connect (w http.ResponseWriter, r *http.Request, userId int) error {
	if !reg.running {
		return errors.New("WS server is not running")
	}

	c, e := upgrader.Upgrade(w, r, nil)
	if e != nil {
		return e
	}

	reg.lock.Lock()
	defer reg.lock.Unlock()

	conn := newConn(c, reg.ids.Next(), userId)
	reg.proto.Connect(conn)

	go func () {
		for conn.alive {
			t, m, e := c.ReadMessage()
			if e == nil && t != websocket.TextMessage {
				e = errors.New("wrong message type")
			}
			if e != nil {
				conn.alive = false
				log.Println(e)
				break
			}

			reg.proto.TakeRequest(conn, m)
		}

		reg.Disconnect(conn.id)
	}()

	return nil
}

func (reg *Registry) Disconnect (id int) {
	if !reg.running {
		return
	}

	reg.lock.Lock()
	defer reg.lock.Unlock()

	conn := reg.conns[id]
	if conn == nil {
		return
	}

	conn.Close()
	reg.proto.Disconnect(id)
	delete(reg.conns, id)
}

func (reg *Registry) Stop () {
	reg.lock.Lock()
	defer reg.lock.Unlock()

	if !reg.running {
		return
	}

	message := []byte("server is stopping")
	reg.running = false
	for id, conn := range reg.conns {
		reg.proto.Disconnect(id)
		conn.c.WriteControl(websocket.CloseMessage, message, time.Now().Add(5 * time.Second))
		conn.Close()
	}
	reg.conns = make(map[int]*connRec)
}
