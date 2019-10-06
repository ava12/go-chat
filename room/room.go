package room

type Entry struct {
	Id int `json:"id"`
	Name string `json:"name"`
}

type Registry interface {
	ListRooms () []Entry
	CreateRoom (name string) (id int, e error)
	Room (id int) (Entry, bool)
}
