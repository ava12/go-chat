package ws

import (
	"errors"
	"log"
	"net/http"
	"time"
	"github.com/gorilla/websocket"
	"github.com/ava12/go-chat/proto"
)

var upgrader = websocket.Upgrader {
	HandshakeTimeout: 5 * time.Second,
	ReadBufferSize: 1024,
	WriteBufferSize: 1024,
}

type connRec struct {
	c *websocket.Conn
	remoteAddr string
	id, userId int
	alive bool
}

func New (w http.ResponseWriter, r *http.Request, p *proto.Proto, id, userId int) (*connRec, error) {
	c, e := upgrader.Upgrade(w, r, nil)
	if e != nil {
		return nil, e
	}

	conn := &connRec {c, r.RemoteAddr, id, userId, true}
	p.Connect(conn)

	go func () {
		for conn.alive {
			t, m, e := c.ReadMessage()
			if e == nil && t != websocket.TextMessage {
				e = errors.New("wrong WS message type")
			}
			if e != nil {
				conn.alive = false
				log.Printf("u%dc%d (%s) %s\n", conn.userId, conn.id, conn.remoteAddr, e.Error())
				break
			}

			p.TakeRequest(conn, m)
		}

		p.Disconnect(conn.id)
	}()

	return conn, nil
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
	c.c = nil
}

func (c *connRec) IsAlive () bool {
	return c.alive
}
