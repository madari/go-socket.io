package socketio

import (
	"net"
	"net/http"
)

type TransportType int

const (
	PollingTransport TransportType = iota
	StreamingTransport
)

type Transport struct {
	Hijack      func(http.ResponseWriter, *http.Request, func(Socket)) error
	Name        string
	PostEncoded bool
	Type        TransportType
}

var DefaultTransports = []*Transport{
	// Flashsocket,
	HTMLFile,
	JSONPPolling,
	Websocket,
	XHRPolling,
}

type Socket interface {
	net.Conn
	Receive(*[]byte) error
}
