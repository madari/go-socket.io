package socketio

import (
	"http"
	"os"
	"io"
	"net"
	"fmt"
)

// The configuration set
type xhrPollingTransportConfig struct {
	To int64
}

// The public function to create a configuration set.
// to is the maximum outstanding time until a reconnection is forced.
func XHRPollingTransportConfig(to int64) TransportConfig {
	return &xhrPollingTransportConfig{To: to}
}

// Returns the resource name
func (tc *xhrPollingTransportConfig) Resource() string {
	return "xhr-polling"
}

// Creates a new transport to be used with c
func (tc *xhrPollingTransportConfig) newTransport(c *Conn) (ts transport) {
	return &xhrPollingTransport{
		tc:   tc,
		conn: c,
	}
}

// Implements the transport interface for XHR/long-polling transports
type xhrPollingTransport struct {
	tc        *xhrPollingTransportConfig
	rwc       io.ReadWriteCloser
	conn      *Conn
	connected bool
}

// String returns the verbose representation of the transport instance
func (t *xhrPollingTransport) String() string {
	return t.tc.Resource()
}

// Config returns the transport configuration used
func (t *xhrPollingTransport) Config() TransportConfig {
	return t.tc
}

// Handles a http connection & request pair. If the method is POST, a new message
// is expected, otherwise a new long-polling connection is established.
func (t *xhrPollingTransport) handle(conn *http.Conn, req *http.Request) (err os.Error) {
	switch req.Method {
	case "GET":
		rwc, _, err := conn.Hijack();
		if err == nil {
			t.rwc = rwc
			t.connected = true
			go t.closer()

			t.conn.onConnect()
		}
		return

	case "POST":
		if msg := req.FormValue("data"); msg != "" {
			t.conn.onMessage([]byte(msg))
		}
	}

	return
}

// Closer tries to read from the i/o connection until an error is encountered
// or the timeout has been reached and then closes the connection.
func (t *xhrPollingTransport) closer() {
	buf := make([]byte, 1)
	rwc := t.rwc

	if t.tc.To > 0 {
		rwc.(*net.TCPConn).SetReadTimeout(t.tc.To)
	}

	for _, err := rwc.Read(buf); rwc == t.rwc; {
		if err != nil && err != os.EAGAIN && err != os.E2BIG {
			t.Close()
			return
		}
	}
}

// Write sends a single message to the wire and closes the connection
func (t *xhrPollingTransport) Write(p []byte) (n int, err os.Error) {
	if !t.connected {
		return 0, ErrNotConnected
	}

	n, err = fmt.Fprintf(t.rwc,
		"HTTP/1.0 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s",
		len(p), p)

	t.Close()
	return
}

// Close tears the connection down and invokes the onDisconnect on the
// owner connection
func (t *xhrPollingTransport) Close() (err os.Error) {
	if t.connected {
		t.connected = false
		t.conn.onDisconnect()
		err = t.rwc.Close()
	}

	return
}
