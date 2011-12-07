package socketio

import (
	"errors"
	"net/http"
	"websocket"
)

var errWebsocketHandshake = errors.New("websocket handshake error")

// The websocket transport.
type websocketTransport struct {
	rtimeout int64 // The period during which the client must send a message.
	wtimeout int64 // The period during which a write must succeed.
}

// Creates a new websocket transport with the given read and write timeouts.
func NewWebsocketTransport(rtimeout, wtimeout int64) Transport {
	return &websocketTransport{rtimeout, wtimeout}
}

// Returns the resource name.
func (t *websocketTransport) Resource() string {
	return "websocket"
}

// Creates a new socket that can be used with a connection.
func (t *websocketTransport) newSocket() socket {
	return &websocketSocket{t: t}
}

// websocketTransport implements the transport interface for websockets
type websocketSocket struct {
	t         *websocketTransport // the transport configuration
	ws        *websocket.Conn     // the websocket connection
	connected bool                // used internally to represent the connection state
	close     chan byte
}

// Transport returns the transport the socket is based on.
func (s *websocketSocket) Transport() Transport {
	return s.t
}

// String returns the verbose representation of the socket.
func (s *websocketSocket) String() string {
	return s.t.Resource()
}

// Accepts a http connection & request pair. It upgrades the connection and calls
// proceed if succesfull.
//
// TODO: Remove the ugly channels and timeouts. They should not be needed!
func (s *websocketSocket) accept(w http.ResponseWriter, req *http.Request, proceed func()) (err error) {
	if s.connected {
		return ErrConnected
	}

	f := func(ws *websocket.Conn) {
		err = nil
		ws.SetReadTimeout(s.t.rtimeout)
		ws.SetWriteTimeout(s.t.wtimeout)
		s.connected = true
		s.ws = ws
		s.close = make(chan byte)
		defer close(s.close)

		proceed()

		// must block until closed
		<-s.close
	}

	err = errWebsocketHandshake
	websocket.Handler(f).ServeHTTP(w, req)
	return
}

func (s *websocketSocket) Read(p []byte) (int, error) {
	if !s.connected {
		return 0, ErrNotConnected
	}

	return s.ws.Read(p)
}

func (s *websocketSocket) Write(p []byte) (int, error) {
	if !s.connected {
		return 0, ErrNotConnected
	}

	return s.ws.Write(p)
}

func (s *websocketSocket) Close() error {
	if !s.connected {
		return ErrNotConnected
	}

	s.connected = false
	go func() {
		select {
		case s.close <- 1:
		default:
		}
	}()

	return s.ws.Close()
}
