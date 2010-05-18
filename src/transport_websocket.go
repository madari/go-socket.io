package socketio

import (
	"http"
	"os"
	"websocket"
)

// websocketTransportConfig is the configuration set
type websocketTransportConfig struct {
	To int64 // the maximum outstanding time until a reconnection is forced
	Draft75 bool // use draft75 or the bleeding edge
}

// WebsocketTransportConfig is the public function to create a configuration set.
// to is the maximum outstanding time until a reconnection is forced.
func WebsocketTransportConfig(to int64, draft75 bool) TransportConfig {
	return &websocketTransportConfig{
		To: to,
		Draft75: draft75,
	}
}

// Returns the resource name
func (tc *websocketTransportConfig) Resource() string {
	return "websocket"
}

// Creates a new transport to be used with c
func (tc *websocketTransportConfig) newTransport(conn *Conn) (ts transport) {
	return &websocketTransport{
		tc:   tc,
		conn: conn,
	}
}

// websocketTransport implements the transport interface for websockets
type websocketTransport struct {
	tc        *websocketTransportConfig // the transport configuration
	ws        *websocket.Conn           // the websocket connection
	conn      *Conn                     // owner socket.io connection
	connected bool                      // used internally to represent the connection state
}

// Config returns the transport configuration used
func (t *websocketTransport) Config() TransportConfig {
	return t.tc
}

// String returns the verbose representation of the transport instance
func (t *websocketTransport) String() string {
	return t.tc.Resource()
}

// Handles a http connection & request pair
func (t *websocketTransport) handle(conn *http.Conn, req *http.Request) (err os.Error) {
	go func() {

		f := func(ws *websocket.Conn) {
			t.ws = ws
			t.connected = true
			t.conn.onConnect()

			// try to read forever and close the socket after read fails
			t.reader()
		}
		
		// upgrade the connection
		if t.tc.Draft75 {
			websocket.Draft75Handler(f).ServeHTTP(conn, req)
		} else {
			websocket.Handler(f).ServeHTTP(conn, req)
		}
	}()

	return
}

// Reader reads data from the websocket and handles timeouts.
// It passes the read messages to the owner's onMessage handler
// When it encounters an EOF or other errors on the line,
// it will call the Close function.
func (t *websocketTransport) reader() {
	buf := make([]byte, 2048)

	defer t.Close()

	// set timeout if specified by the configuration
	if t.tc.To > 0 {
		t.ws.SetReadTimeout(t.tc.To)
	}

	for {
		nr, err := t.ws.Read(buf)
		if err != nil {
			// if an unknown error is encountered, then return
			// and close the connection, otherwise keep on reading
			if err != os.E2BIG && err != os.EAGAIN {
				return
			} else {
				// TODO: handle os.E2BIG properly so that the big messages
				//       don't get discarded
				continue
			}
		}

		if nr > 0 {
			t.conn.onMessage(buf[0:nr])
		}
	}
}

// Writes p to the connection and returns the number of bytes written n
// and an nil error if write succeeded
func (t *websocketTransport) Write(p []byte) (n int, err os.Error) {
	if !t.connected {
		return 0, os.EOF
	}

	n, err = t.ws.Write(p)
	return
}

// Closes the connection and calls the owner's onDisconnect handler.
func (t *websocketTransport) Close() (err os.Error) {
	if t.connected {
		t.connected = false
		t.conn.onDisconnect()
		err = t.ws.Close()
	}

	return
}
