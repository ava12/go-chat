package access

type PermFlags = int

const (
	ListRoomsPerm = 1 << iota
	CreateRoomPerm
	AllGlobalPerms = ListRoomsPerm | CreateRoomPerm
)

const (
	ReadPerm = 1 << iota
	WritePerm
	AllRoomPerms = ReadPerm | WritePerm
)

type Controller interface {
	GlobalPerms (userId int) PermFlags
	RoomPerms (userId, roomId int) PermFlags
	HasGlobalPerm (userId, perm int) bool
	HasRoomPerm (userId, roomId, perm int) bool
	NewRoom (userId, roomId int)
}
