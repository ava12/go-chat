
function Xhr (hostname, errHandler) {
	this.xhr = new XMLHttpRequest()
	this.hostname = (hostname ? hostname : '')
	this.errHandler = errHandler
}

Xhr.prototype.encodeParamValue = function (value) {
	return encodeURIComponent(value).replace(/[!'()*]/g, function(c) {
		return '%' + c.charCodeAt(0).toString(16);
	}).replace('%20', '+')
}

Xhr.prototype.encodeParam = function (name, value) {
	switch (typeof value) {
		case 'object':
			var params = []
			for (var n in value) if (value.hasOwnProperty(n)) {
				params.push(this.encodeParam(name + '[' + n + ']', value[n]))
			}
			return params.join('&')

		case 'boolean':
			value = Number(value)

		// noinspection FallThroughInSwitchStatementJS
		default:
			return name + '=' + this.encodeParamValue(value)
	}
}

Xhr.prototype.encodeData = function (data) {
	if (!data) {
		return ''
	}

	var params = []
	for (var name in data) if (data.hasOwnProperty(name)) {
		params.push(this.encodeParam(name, data[name]))
	}
	return params.join('&')
}

Xhr.prototype.getJsonResponse = function () {
	return JSON.parse(this.xhr.responseText)
}

Xhr.prototype.query = function (method, url, content, contentType, handler, errHandler) {
	if (!errHandler) {
		errHandler = this.errHandler
	}

	var t = this
	this.xhr.onloadend = function () {
		if (t.xhr.status < 200 || t.xhr.status >= 300) {
			if (errHandler) {
				errHandler(t)
				return
			}

			var message = (t.xhr.status > 0 ? '' + t.xhr.status + ' ' + t.xhr.statusText : 'cannot connect to the server')
			alert(message)
			return
		}

		if (handler) {
			handler(t)
		} else if (handler != undefined) {
			location.reload()
		}
	}

	this.xhr.open(method, url)
	if (content) {
		this.xhr.setRequestHeader('Content-Type', contentType)
		this.xhr.send(content)
	} else {
		this.xhr.send()
	}
}

Xhr.prototype.get = function (path, data, handler, errHandler) {
	this.query('GET', this.hostname + path + '?' + this.encodeData(data), null, null, handler, errHandler)
}

Xhr.prototype.post = function (path, data, handler, errHandler) {
	this.query('POST', this.hostname + path, this.encodeData(data), 'application/x-www-form-urlencoded', handler, errHandler)
}
