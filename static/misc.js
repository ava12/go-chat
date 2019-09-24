window.onerror = function (e) {
	alert('Ошибка: ' + e)
	throw e
}

function formatTime (t, f) {
	if (f == undefined) {
		f = '%y-%m-%d %H:%M:%S'
	}

	var lz = function (s, w, p) {
		w = w || 2
		p = p || '000'
		return (p + s).substr(-w)
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
			case 's': m = lz(t.getMilliseconds(), 3); break
		}
		result.push(m)
	}

	return result.join('')
}

function dumpVar (v, indent) {
	indent = indent || ''
	switch (typeof v) {
		case 'bool': return (v ? 'true' : 'false')
		case 'string': return dumpString(v)
		case 'object': return '\n' + dumpObject(v, indent + '  ')
		default: return String(v)
	}
}

function dumpString (s) {
	return '"' + s.replace(/[\0-\x1f\\"]/g, function (match) {
		if (dumpStringCtl[match] != undefined) {
			return '\\' + dumpStringCtl[match]
		} else {
			return '\\x' + ('0' + match.charCodeAt(0).toString(16)).substr(-2)
		}
	}) + '"'
}

var dumpStringCtl = {'\b': 'b', '\t': 't', '\n': 'n', '\f': 'f', '\r': 'r', '\\': '\\', '"': '"'}

function dumpObject (o, indent) {
	var result = []
	for (var key in o) {
		result.push(indent + dumpString(key) + ': ' + dumpVar(o[key], indent))
	}
	return result.join('\n')
}


function Logger (timeFormat, maxItems) {
	this.timeFormat = (timeFormat ? timeFormat : '%H:%M:%S.%s')
	this.maxItems = (maxItems ? maxItems : 10)
	this.items = []
}

Logger.prototype.log = function (typ, name, value) {
	var ts = new Date()
	var tt = formatTime(ts, this.timeFormat)
	this.items.push({
		time: ts,
		timeText: tt,
		typ: typ,
		name: name,
		value: value,
		dump: tt + '\n' + dumpVar(value)
	})
	if (this.items.length > this.maxItems) {
		this.items.shift()
	}
}

Logger.prototype.last = function () {
	return (this.items.length ? this.items[this.items.length - 1] : {})
}
