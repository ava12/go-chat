
/** @see date(1) */
function formatTime (t, f) {
	if (f == undefined) {
		f = '%y-%m-%d %H:%M:%S'
	}

	var lz = function (s, p) {
		if (p == undefined) {
			p = '0'
		}
		return (p + s).substr(-2)
	}

	var matches = String(f).match(/%.|[^%]+/g)
	var result = []
	for (var i = 0; i < matches.length; i++) {
		var m = matches[i]
		if (m.charAt(0) != '%') {
			result.push(m)
			continue
		}

		switch (m.charAt(1)) {
			case 'y': m = lz(t.getFullYear() % 100); break
			case 'm': m = lz(t.getMonth() + 1); break
			case 'd': m = lz(t.getDate()); break
			case 'e': m = lz(t.getDate(), ' '); break
			case 'H': m = lz(t.getHours()); break
			case 'M': m = lz(t.getMinutes()); break
			case 'S': m = lz(t.getSeconds()); break
		}
		result.push(m)
	}

	return result.join('')
}

function SortedList (idName, keyName) {
	this.idName = (idName ? idName : 'id')
	this.keyName = (keyName ? keyName : 'name')
	this.items = []
}

SortedList.prototype.clear = function () {
	this.items.splice(0, this.items.length)
}

SortedList.prototype.add = function (newItem) {
	var id = newItem[this.idName]
	var key = newItem[this.keyName]

	for (var i = 0; i < this.items.length; i++) {
		var item = this.items[i]
		if (item[this.idName] == id) {
			return
		}

		if (item[this.keyName] > key) {
			this.items.splice(i, 0, newItem)
			return
		}
	}

	this.items.push(newItem)
}

SortedList.prototype.remove = function (id) {
	for (var i = 0; i < this.items.length; i++) {
		if (this.items[i][this.idName] == id) {
			this.items.splice(i, 1)
			return
		}
	}
}

SortedList.prototype.cutHead = function (cnt) {
	if (cnt > 0) {
		this.items.splice(0, cnt)
	}
}


function RoomPerm (flags) {
	this.flags = flags
}

RoomPerm.prototype.canRead = function () {
	return !!(this.flags & 1)
}

RoomPerm.prototype.canWrite = function () {
	return !!(this.flags & 2)
}


function GlobalPerm (flags) {
	this.flags = flags
}

RoomPerm.prototype.canListRooms = function () {
	return !!(this.flags & 1)
}

RoomPerm.prototype.canCreateRoom = function () {
	return !!(this.flags & 2)
}


function User (id, name, color) {
	this.id = +id
	this.name = name
	this.color = color
}


function Room (id, name, perm, isIn) {
	this.id = +id
	this.name = name
	this.isIn = !!isIn
	this.setPerm(perm)
	this.users = new SortedList()

	this.newMessage = false
	this.messages = []
	this.lastId = 0
	this.newMessages = new SortedList('id', 'id')
}

Room.prototype.setPerm = function (perm) {
	this.perm = (typeof perm == 'object' ? perm : new RoomPerm(perm))
}

Room.prototype.addUser = function (user) {
	this.users.add(user)
}

Room.prototype.removeUser = function (userId) {
	this.users.remove(userId)
}

Room.prototype.userEnter = function (user) {
	this.users.add(user)
	this.isIn = true
}

Room.prototype.leave = function () {
	this.isIn = false
	this.newMessage = false
	this.newMessages.clear()
	this.users.clear()
	this.messages.splice(0, this.messages.length)
	this.lastId = 0
}

Room.prototype.untouch = function () {
	if (this.isIn) {
		this.newMessage = true
	}
}

Room.prototype.touch = function () {
	this.newMessage = false
}

Room.prototype.addMessage = function (message, flagNew) {
	var nextId = (this.messages.length ? this.messages[this.messages.length - 1].id + 1 : 1)

	if (message.id < nextId) {
		return
	}

	if (message.id != nextId) {
		this.newMessages.add(message)
		return
	}

	this.messages.push(message)
	this.newMessage = this.newMessage || flagNew

	nextId++
	var headLen = 0
	for (var i = 0; i < this.newMessages.length; i++) {
		message = this.newMessages[i]
		if (message.id != nextId) {
			break
		}

		this.messages.push(message)
		nextId++
		headLen++
	}

	this.newMessages.cutHead(headLen)
}

Room.prototype.shownMessageId = function () {
	return (this.messages.length ? this.messages[this.messages.length - 1].id : 0)
}


function Message (id, roomId, user, timestamp, text) {
	this.id = +id
	this.roomId = +roomId
	this.user = user
	this.time = new Date(timestamp * 1000)
	this.timeText = formatTime(this.time, '%e.%m %H:%M:%S')
	this.text = text
}


function Chat () {
	this.userId = 0
	this.userPerm = 0
	this.users = {}
	this.pendingUsers = {} // {userId: {roomId: [message]}}
	this.rooms = {}
	this.roomList = new SortedList()

	this.currentRoom = null
	this.currentRoomId = 0
}

Chat.prototype.reset = function () {
	var user = this.users[this.userId]
	this.users = {}
	if (user) {
		this.users[user.id] = user
	}
	this.pendingUsers = {}
	this.rooms = {}
	this.roomList = new SortedList()
	this.currentRoom = null
	this.currentRoomId = 0
}

Chat.prototype.resetUser = function () {
	this.userId = 0
	this.userPerm = 0
	this.users = {}
}

Chat.prototype.setUserId = function (id, globalPerm) {
	this.userId = id
	this.userPerm = (typeof globalPerm == 'object' ? globalPerm : new GlobalPerm(globalPerm))
}

Chat.prototype.addUser = function (user) {
	this.users[user.id] = user
	var pending = this.pendingUsers[user.id]
	if (!pending) {
		return
	}

	for (var roomId in pending) {
		var room = this.rooms[roomId]
		var flagNew = (roomId != this.currentRoomId)
		for (var i = 0; i < pending[roomId].length; i++) {
			var message = pending[roomId][i]
			message.user = user
			room.addMessage(message, flagNew)
		}
	}

	delete this.pendingUsers[user.id]
}

Chat.prototype.addRoom = function (room) {
	if (this.rooms[room.id] != undefined) {
		return
	}

	this.rooms[room.id] = room
	this.roomList.add(room)
}

Chat.prototype.getUser = function (userId) {
	return this.users[userId]
}

Chat.prototype.getRoom = function (roomId) {
	return this.rooms[roomId]
}

Chat.prototype.enterRoom = function (roomId, user) {
	if (user == undefined) {
		user = this.users[this.userId]
	}
	var room = this.rooms[roomId]
	if (user.id == this.userId || room.isIn) {
		room.userEnter(user)
		this.currentRoomId = room.id
		this.currentRoom = room
		if (user.id == this.userId) {
			room.touch()
		}
	}
}

Chat.prototype.leaveRoom = function (roomId, userId) {
	if (roomId == undefined) roomId = this.currentRoomId
	if (userId == undefined) userId = this.userId
	room = this.rooms[roomId]
	if (!room) {
		return
	}

	room.removeUser(userId)
	if (userId == this.userId) {
		this.currentRoomId = 0
		this.currentRoom = null
		room.leave()
	}
}

Chat.prototype.pending = function (userId, roomId, message) {
	if (!this.rooms[roomId]) {
		return
	}

	if (!this.pendingUsers[userId]) {
		this.pendingUsers[userId] = {roomId: []}
	}
	if (!this.pendingUsers[userId][roomId]) {
		this.pendingUsers[userId][roomId] = []
	}
	this.pendingUsers[userId][roomId].push(message)
}
