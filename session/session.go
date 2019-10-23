package session

type Session interface {
	Id () string
	UserId () int
	Ttl () int64
	Touch ()
	Expired () bool
}

type Registry interface {
	Session (id string) Session
	Touch (id string) bool
	NewSession (userId int) Session
	Sweep ()
	Delete (id string)
}
