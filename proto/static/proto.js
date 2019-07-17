
function ChatProto (callbacks, conn) {
	this.conn = null
	this.on = {
		error: null, // function (message)
		whoami: null, // function (user, globalPerm)
		listRooms: null, // function (rooms)
		inRooms: null, // function (rooms)
		newRoom: null, // function (room)
		enter: null, // function (roomId, user, roomPerm)
		leave: null, // function (roomId, userId)
		listUsers: null, // function (roomId, users)
		listMessages: null, // function (roomId, firstMessageId, messages)
		userInfo: null, // function (user)
		textMessage: null, // function (roomId, messageId, userId, timestamp, text)
		response: null, // function (responseType, responseBody)
		connError: null // function (message)
	}

	if (callbacks) {
		this.setCallbacks(callbacks)
	}

	if (conn) {
		this.connect(conn)
	}
}

ChatProto.prototype.perm = {
	global: {
		listRooms: 1,
		createRoom: 2
	},
	room: {
		read: 1,
		write: 2
	}
}

ChatProto.prototype.messageTypes = {
	text: 1
}

ChatProto.prototype.responseMap = { // {response: [callback, (arg... | '*')]}
	whoami: ['whoami', 'user', 'perm'],
	'list-rooms': ['listRooms', 'rooms'],
	'in-rooms': ['inRooms', 'rooms'],
	enter: ['enter', 'roomId', 'user', 'perm'],
	leave: ['leave', 'roomId', 'userId'],
	'new-room': ['newRoom', '*'],
	'list-users': ['listUsers', 'roomId', 'users'],
	'list-messages': ['listMessages', 'roomId', 'firstMessageId', 'messages'],
	'user-info': ['userInfo', '*'],
	error: ['error', 'message']
}

ChatProto.prototype.setCallbacks = function (callbacks) {
	for (var name in callbacks) {
		if (this.on.hasOwnProperty(name)) {
			this.on[name] = callbacks[name]
		}
	}

}

ChatProto.prototype.connect = function (conn) {
	if (this.conn) {
		this.disconnect()
	}

	this.conn = conn
	var t = this
	this.conn.connect(
		function (response) {
			t.takeResponse(response)
		},
		function (message) {
			t.takeError(message)
		}
	)
}

ChatProto.prototype.disconnect = function () {
	if (!this.conn) {
		return
	}

	this.conn.disconnect()
	this.conn = null
}

ChatProto.prototype.takeResponse = function (response) {
	var name, args

	switch (response.response) {
		case 'message':
			var b = response.body
			args = [b.roomId, b.messageId, b.userId, b.timestamp]

			switch (b.data.messageType) {
				case this.messageTypes.text:
					name = 'textMessage'
					args.push(b.data.data.text)
				break
			}
		break

		default:
			var def = this.responseMap[response.response]
			if (def) {
				name = def[0]
				for (var i = 1; i < def.length; i++) {
					args.push(def[i] == '*' ? response.body : response.body[def[i]])
				}
			}
	}

	if (name && this.on[name]) {
		this.on[name].apply(null, args)
		return
	}

	if (this.on.response) {
		this.on.response(response.response, response.body)
	}
}

ChatProto.prototype.takeError = function (message) {
	this.disconnect()
	if (this.on.connError) {
		this.on.connError(message)
	}
}

ChatProto.prototype.sendRequest (request) {
	if (!this.conn) {
		this.takeError('not connected to server')
		return
	}

	this.conn.send(request)
}

ChatProto.prototype.send = function (requestType, requestBody) {
	this.sendRequest({request: requestType, body: requestBody})
}

ChatProto.prototype.sendWhoami = function () {
	this.send('whoami')
}

ChatProto.prototype.sendListRooms = function () {
	this.send('list-rooms')
}

ChatProto.prototype.sendInRooms = function () {
	this.send('in-rooms')
}

ChatProto.prototype.sendEnter = function (roomId) {
	this.send('enter', {roomId: roomId})
}

ChatProto.prototype.sendLeave = function (roomId) {
	this.send('leave', {roomId: roomId})
}

ChatProto.prototype.sendListUsers = function (roomId) {
	this.send('list-users', {roomId: roomId})
}

ChatProto.prototype.sendListMessages = function (roomId, firstMessageId, messageCnt) {
	this.send('list-messages', {roomId: roomId, firstMessageId: firstMessageId, messageCnt: messageCnt})
}

ChatProto.prototype.sendUserInfo = function (userId) {
	this.send('user-info', {userId: userId})
}

ChatProto.prototype.sendTextMessage = function (roomId, text) {
	this.send('message', {roomId: roomId, messageType: this.messageTypes.text, data: {text: text}})
}
