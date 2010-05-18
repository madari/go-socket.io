package socketio

import (
	"http"
	"os"
	"io"
	"net"
	"fmt"
)

// The configuration set
type xhrMultipartTransportConfig struct {
	To int64
}

// The public function to create a configuration set.
// to is the maximum outstanding time until a reconnection is forced.
func XHRMultipartTransportConfig(to int64) TransportConfig {
	return &xhrMultipartTransportConfig{To: to}
}

// Returns the resource name
func (tc *xhrMultipartTransportConfig) Resource() string {
	return "xhr-multipart"
}

// Creates a new transport to be used with c
func (tc *xhrMultipartTransportConfig) newTransport(c *Conn) (ts transport) {
	return &xhrMultipartTransport{
		tc:   tc,
		conn: c,
	}
}

// Implements the transport interface for XHR/multipart transports
type xhrMultipartTransport struct {
	tc        *xhrMultipartTransportConfig
	rwc       io.ReadWriteCloser
	conn      *Conn
	connected bool
}

// String returns the verbose representation of the transport instance
func (t *xhrMultipartTransport) String() string {
	return t.tc.Resource()
}

// Config returns the transport configuration used
func (t *xhrMultipartTransport) Config() TransportConfig {
	return t.tc
}

// Handles a http connection & request pair. If the method is POST, a new message
// is expected, otherwise a new multipart connection is established.
func (t *xhrMultipartTransport) handle(conn *http.Conn, req *http.Request) (err os.Error) {
	switch req.Method {
	case "GET":
		if rwc, _, err := conn.Hijack(); err == nil {
			_, err = fmt.Fprint(rwc,
				"HTTP/1.0 200 OK\r\n",
				"Content-Type: multipart/x-mixed-replace; boundary=\"socketio\"\r\n",
				"Connection: keep-alive\r\n\r\n",
				"--socketio\r\n")

			if err != nil {
				rwc.Close()
				return
			}

			t.rwc = rwc
			t.connected = true
			go t.closer()

			t.conn.onConnect()
		}

	case "POST":
		if msg := req.FormValue("data"); msg != "" {
			t.conn.onMessage([]byte(msg))
		}
	}

	return
}

// Closer tries to read from the i/o connection until an error is encountered
// or the timeout has been reached and then closes the connection.
func (t *xhrMultipartTransport) closer() {
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

// Write sends a single multipart message to the wire
func (t *xhrMultipartTransport) Write(p []byte) (n int, err os.Error) {
	if !t.connected {
		return 0, os.EOF
	}

	n, err = fmt.Fprintf(t.rwc,
		"Content-Type: text/plain\r\n\r\n%s\n--socketio\n", p)

	return
}

// Close tears the connection down and invokes the onDisconnect on the
// owner connection
func (t *xhrMultipartTransport) Close() (err os.Error) {
	if t.connected {
		t.connected = false
		t.conn.onDisconnect()
		err = t.rwc.Close()
	}

	return
}
