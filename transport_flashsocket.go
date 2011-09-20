package socketio

var Flashsocket = &Transport{
	Name:   "flashsocket",
	Type:   StreamingTransport,
	Hijack: websocketHijack,
}
