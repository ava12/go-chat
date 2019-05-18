var chatApp
var lastRoomId = 0

function initChat (chat, userName) {
	var users = ['это я', 'Вася', 'Петя', 'Маша']
	var userId = users.indexOf(userName) + 1
	if (userId < 1) {
		users.push(userName)
		userId = users.length
	}

	var time = (new Date()).getTime() / 1000

	var rooms = [
		['просто комната', [], []],
		['моя комната', [userId, 2, 4], [
			[1, 'привет всем'],
			[2, 'йцукен'],
			[4, ':))']
		]],
		['тоже комната', [userId, 3, 4], [
			[3, ':)']
		]]
	]

	chat.userId = userId
	var i, j

	for (i = 0; i < users.length; i++) {
		chat.addUser(new User(i + 1, users[i], (i + 1) == userId ? null : 'col' + (i + 1)))
	}

	for (i = 0; i < rooms.length; i++) {
		var room = new Room(i + 1, rooms[i][0])
		chat.addRoom(room)
		for (j = 0; j < rooms[i][1].length; j++) {
			room.addUser(chat.users[rooms[i][1][j]])
			if (rooms[i][1][j] == userId) {
				room.isIn = true
				room.untouch()
			}
		}
		for (j = 0; j < rooms[i][2].length; j++) {
			var entry = rooms[i][2][j]
			room.messages.push(new Message(j + 1, room.id, chat.users[entry[0]], time++, entry[1]))
		}
		room.lastId = room.messages.length
	}
}

window.onload = function () {
	chatApp = new Vue({
		el: '#content',
		data: {
			userName: 'это я',
			lastRoomId: 0,
			chat: new Chat(),
			messageText: '',
			state: 'login',
			states: {
				login: 'login',
				chat: 'chat'
			}
		},
		methods: {
			login: function () {
				initChat(this.chat, this.userName.trim())
				this.lastRoomId = this.chat.roomList.length
				this.state = this.states.chat
			},

			logout: function () {
				this.state = this.states.login
				this.chat = new Chat()
			},

			rest: function () {
				this.messageText = ''
				chatApp.$nextTick(function () {
					location.hash = this.chat.currentId
					document.getElementById('input').focus()
				})
			},

			newRoom: function () {
				var name = prompt('Название комнаты')
				if (!name) return

				name = name.trim()
				var loName = name.toLowerCase()
				for (var id in this.chat.rooms) {
					if (this.chat.rooms[id].name.toLowerCase() == loName) {
						alert('такая комната уже есть')
						return
					}
				}

				this.lastRoomId++
				var room = new Room(lastRoomId, name)
				this.chat.addRoom(room)
				this.chat.enterRoom(room.id)
				this.rest()
			},

			leaveRoom: function () {
				this.chat.leaveRoom()
				this.messageText = ''
				location.hash = ''
				input.blur()
			},

			selectRoom: function (roomId) {
				this.chat.enterRoom(roomId)
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

				c.currentRoom.lastId++
				var message = new Message(c.currentRoom.lastId, c.currentId, c.users[c.userId], (new Date()).getTime() / 1000, text)
				c.currentRoom.messages.push(message)
				this.rest()
			}
		}
	})
}
