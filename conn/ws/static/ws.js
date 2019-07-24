
WsConn = function (url) {
	if (!url) {
		url = 'ws://' + location.host + '/ws'
	}
	if (url.charAt(0) == '/' && url.charAt(1) != '/') {
		url = '//' + location.host + url
	}
	if (url.charAt(0) == '/') {
		url = 'ws:' + url
	}

	this.url = url
	this.ws = null
	this.running = false
	this.queue = []
	this.messageCallback = null
	this.errorCallback = null
}

WsConn.prototype.cleanup = function () {
	this.ws = null
	this.running = false
	this.queue = []
}

WsConn.prototype.connect = function (messageCallback, errorCallback) {
	this.messageCallback = messageCallback
	this.errorCallback = errorCallback
	if (this.ws) {
		return
	}

	this.ws = new WebSocket(this.url)
	if (!this.ws) {
		this.giveError('cannot connect to ' + this.url)
	}

	var t = this

	this.ws.onopen = function () {
		t.running = true
		for (var i in t.queue) {
			t.ws.send(t.queue[i])
		}
		t.queue = []
	}

	this.ws.onclose = function (e) {
		t.cleanup()
		t.giveError('WS connection is closed: ' + e.code + ' ' + e.reason)
	}

	this.ws.onerror = function (e) {
		t.cleanup()
		t.giveError('WS error: ' + e)
	}

	this.ws.onmessage = function (e) {
		if (typeof e.data != 'string') {
			t.giveError('incorrect WS message type: ' + typeof e.data)
			t.disconnect()
			return
		}

		var data = JSON.parse(e.data)
		if (data == undefined) {
			t.giveError('incorrect WS message')
			t.disconnect()
			return
		}

		t.giveMessage(data)
	}
}

WsConn.prototype.disconnect = function () {
	if (!this.ws) {
		return
	}

	this.ws.close()
	this.cleanup()
}

WsConn.prototype.send = function (data) {
	if (!this.ws) {
		this.giveError('no WS connection')
		return
	}

	if (typeof data != 'string') {
		data = JSON.stringify(data)
	}

	if (this.running) {
		this.ws.send(data)
	} else {
		this.queue.push(data)
	}
}

WsConn.prototype.giveMessage = function (data) {
	if (this.messageCallback) {
		this.messageCallback(data)
	}
}

WsConn.prototype.giveError = function (message) {
	if (this.errorCallback) {
		this.errorCallback(message)
	}
}
