package simple

import (
	"github.com/ava12/go-chat/access"
)

type accessRec struct {}

func NewAccessController () access.Controller {
	return &accessRec {}
}

func (ar *accessRec) GlobalPerms (userId int) access.PermFlags {
	return access.AllGlobalPerms
}

func (ar *accessRec) RoomPerms (userId, roomId int) access.PermFlags {
	return access.AllRoomPerms
}

func (ar *accessRec) HasGlobalPerm (userId int, perm access.PermFlags) bool {
	return (perm & access.AllGlobalPerms != 0)
}

func (ar *accessRec) HasRoomPerm (userId, roomId int, perm access.PermFlags) bool {
	return (perm & access.AllRoomPerms != 0)
}

func (ar *accessRec) NewRoom (userId, roomId int) {}
