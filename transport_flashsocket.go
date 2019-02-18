package socketio

import (
	"http"
	"os"
)

// The flashsocket transport.
type flashsocketTransport struct {
	wsTransport *websocketTransport
}

// Creates a new flashsocket transport with the given read and write timeouts.
func NewFlashsocketTransport(rtimeout, wtimeout int64) Transport {
	return &flashsocketTransport{&websocketTransport{rtimeout, wtimeout}}
}

// Resource returns the resource name.
func (t *flashsocketTransport) Resource() string {
	return "flashsocket"
}

// Creates a new socket that can be used with a connection.
func (t *flashsocketTransport) newSocket() socket {
	return &flashsocketSocket{t: t, s: t.wsTransport.newSocket()}
}

// flashsocketTransport implements the transport interface for flashsockets
type flashsocketSocket struct {
	t *flashsocketTransport // the transport configuration
	s socket
}

// Transport returns the transport the socket is based on.
func (s *flashsocketSocket) Transport() Transport {
	return s.t
}

// String returns the verbose representation of the socket.
func (s *flashsocketSocket) String() string {
	return s.t.Resource()
}

// Accepts a http connection & request pair. It upgrades the connection and calls
// proceed if succesfull.
//
// TODO: Remove the ugly channels and timeouts. They should not be needed!
func (s *flashsocketSocket) accept(w http.ResponseWriter, req *http.Request, proceed func()) (err os.Error) {
	return s.s.accept(w, req, proceed)
}

func (s *flashsocketSocket) Read(p []byte) (int, os.Error) {
	return s.s.Read(p)
}

func (s *flashsocketSocket) Write(p []byte) (int, os.Error) {
	return s.s.Write(p)
}

func (s *flashsocketSocket) Close() os.Error {
	return s.s.Close()
}
