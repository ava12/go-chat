var chatApp
var lastRoomId = 0

function makeUser (data, chat) {
	var col = (data.id == chat.userId ? '' : 'col' + (data.id % 10))
	return new User(data.id, data.name, col)
}

function makeRoom (data, isIn) {
	return new Room(data.id, data.name, data.perm, !!isIn)
}

function initApp (app) {
	var proto = app.proto
	var chat = app.chat

	var textMessageHandler = function (roomId, messageId, userId, timestamp, text) {
		var room = chat.getRoom(roomId)
		if (!room) {
			return
		}

		var user = chat.getUser(userId), message
		if (user) {
			message = new Message(messageId, roomId, user, timestamp, text)
			room.addMessage(message, room.id != chat.currentRoomId)
			return
		}

		proto.sendUserInfo(userId)
		user = makeUser({id: userId, name: '???'}, chat)
		message = new Message(messageId, roomId, user, timestamp, text)
		chat.pending(userId, roomId, message)
	}

	var callbacks = {
		afterRecv: function (response) {
			var typ = (response.response == 'error' ? 'error' : 'response')
			app.logger.log(typ, response.response, response)
		},

		beforeSend: function (request) {
			app.logger.log('request', request.request, request)
		},

		connError: function (message) {
			app.errorText = message
			app.state = app.states.disconnected
			chat.reset()
		},

		whoami: function (user, globalPerm) {
			chat.setUserId(user.id, globalPerm)
			chat.addUser(makeUser(user, chat))
			chat.setUserId(user.id, globalPerm)
		},
		listRooms: function (rooms) {
			for (var i = 0; i < rooms.length; i++) {
				chat.addRoom(makeRoom(rooms[i]))
			}
		},
		inRooms: function (rooms) {
			for (var i = 0; i < rooms.length; i++) {
				var room = chat.getRoom(rooms[i].id)
				room.setPerm(rooms[i].perm)
				room.userEnter(chat.getUser(chat.userId))
				app.proto.sendListMessages(room.id, -50, 50)
			}
		},
		newRoom: function (room) {
			chat.addRoom(makeRoom(room))
		},
		enter: function (roomId, user) {
			user = makeUser(user, chat)
			chat.addUser(user)
			chat.enterRoom(roomId, user)
		},
		leave: function (roomId, userId) {
			if (userId == chat.userId && roomId == chat.currentRoomId) {
				location.hash = ''
			}
			chat.leaveRoom(roomId, userId)
		},
		listUsers: function (roomId, users) {
			for (var i = 0; i < users.length; i++) {
				var user = makeUser(users[i], chat)
				chat.addUser(user)
				chat.enterRoom(roomId, user)
			}
		},
		userInfo: function (user) {
			chat.addUser(makeUser(user, chat))
		},
		roomInfo: function (room) {
			chat.addRoom(makeRoom(room))
		},
		textMessage: function (roomId, messageId, userId, timestamp, text) {
			var room = chat.getRoom(roomId)
			if (!room) {
				return
			}

			var smi = room.shownMessageId()
			if (!smi && messageId > 1) {
				return
			}

			textMessageHandler(roomId, messageId, userId, timestamp, text)
			if (smi + 1 != messageId) {
				proto.sendListMessages(roomId, smi + 1, messageId - smi - 1)
			} else {
				app.scroll()
			}
		},
		listMessages: function (roomId, firstId, messages) {
			if (!messages.length) {
				return
			}

			var room = chat.getRoom(roomId)
			if (!room) {
				return
			}

			var flagNew = (roomId != chat.currentRoomId)
			for (var i = 0; i < messages.length; i++) {
				var m = messages[i]
				if (m.data.messageType != proto.messageTypes.text) {
					continue
				}

				textMessageHandler(m.roomId, m.messageId, m.userId, m.timestamp, m.data.data.text)
			}

			app.scroll()
		}
	}

	proto.setCallbacks(callbacks)
}

function createApp () {
	var app = new Vue({
		el: '#content',
		data: {
			userName: '',
			lastRoomId: 0,
			chat: new Chat(),
			proto: new ChatProto(),
			messageText: '',
			errorText: '',
			logger: new Logger(),
			showLogger: false,
			loggerDump: null,
			state: 'init',
			states: {
				init: 'init',
				login: 'login',
				connect: 'connect',
				chat: 'chat',
				disconnected: 'disconnected'
			},
			transitionalStates: {
				init: 'Инициализация…',
				connect: 'Подключение…',
				disconnected: 'Подключение к серверу разорвано'
			}
		},
		methods: {
			run: function () {
				var t = this
				;(new Xhr()).post('/whoami', null, function (xhr) {
					var response = xhr.getJsonResponse()
					if (!response.user) {
						t.state = t.states.login
					} else {
						t.userName = response.user.name
						t.state = t.states.chat
						t.connect()
					}
				})
			},

			expandLog: function () {
				this.showLogger = true
			},

			collapseLog: function () {
				this.loggerDump = null
				this.showLogger = false
			},

			openDump: function (ev) {
				this.loggerDump = ev.target.dataset.dump
			},

			closeDump: function () {
				this.loggerDump = null
			},

			login: function () {
				var t = this
				t.state = t.states.connect
				;(new Xhr()).post('/login', {name: t.userName}, function (xhr) {
					var response = xhr.getJsonResponse()
					if (!response.success) {
						alert('Не удалось подключиться')
						t.state = t.states.login
					} else {
						t.userName = response.user.name
						t.state = t.states.chat
						t.connect()
					}
				})
			},

			logout: function () {
				;(new Xhr()).post('/logout', null, false)
				this.proto.disconnect()
				this.chat.reset()
				this.chat.resetUser()
				this.userName = ''
				this.state = this.states.login
			},

			connect: function () {
				this.errorText = ''
				this.proto.connect(new WsConn())
				this.proto.sendWhoami()
				this.proto.sendListRooms()
				this.proto.sendInRooms()
			},

			scroll: function () {
				if (!this.chat.currentRoomId) {
					return
				}

				app.$nextTick(function () {
					location.hash = ''
					location.hash = this.chat.currentRoomId
					document.getElementById('input').focus()
				})
			},

			rest: function () {
				this.messageText = ''
				document.getElementById('input').focus()
				this.scroll()
			},

			newRoom: function () {
				var name = prompt('Название комнаты')
				name = name.trim()
				if (!name) return

				this.proto.sendNewRoom(name)
				this.rest()
			},

			leaveRoom: function () {
				this.proto.sendLeave(this.chat.currentRoomId)
				input.blur()
			},

			selectRoom: function (roomId) {
				var room = this.chat.getRoom(roomId)
				if (room.isIn) {
					this.chat.enterRoom(roomId)
					this.proto.sendListUsers(roomId)
				} else {
					this.proto.sendEnter(roomId)
					this.proto.sendRoomInfo(roomId)
					this.proto.sendListUsers(roomId)
					this.proto.sendListMessages(roomId, -50, 50)
				}
				this.rest()
			},

			addNewline: function () {
				this.messageText += '\n'
				document.getElementById('input').focus()
			},

			sendMessage: function () {
				var c = this.chat
				if (!c.currentRoom) {
					this.messageText = ''
					return
				}

				var text = this.messageText.trim()
				if (text == '') return

				this.proto.sendTextMessage(c.currentRoomId, text)
				this.rest()
			}
		}
	})

	return app
}

window.onload = function () {
	location.hash = ''
	chatApp = createApp()
	initApp(chatApp)
	chatApp.run()
}
