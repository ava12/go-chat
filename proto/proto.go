package proto

import (
	"github.com/ava12/go-chat/conn"
)

type Proto interface {
	Connect (c conn.Conn)
	Disconnect (connId int)
	Stop ()
	TakeRequest (c conn.Conn, r []byte)
}
