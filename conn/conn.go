package conn

type Conn interface {
	Id () int
	UserId () int
	Send (m []byte)
	Close ()
	IsAlive () bool
}
