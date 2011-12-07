package socketio

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
)

// The xhr-multipart transport.
type xhrMultipartTransport struct {
	rtimeout int64 // The period during which the client must send a message.
	wtimeout int64 // The period during which a write must succeed.
}

// Creates a new xhr-multipart transport with the given read and write timeouts.
func NewXHRMultipartTransport(rtimeout, wtimeout int64) Transport {
	return &xhrMultipartTransport{rtimeout, wtimeout}
}

// Returns the resource name.
func (t *xhrMultipartTransport) Resource() string {
	return "xhr-multipart"
}

// Creates a new socket that can be used with a connection.
func (t *xhrMultipartTransport) newSocket() socket {
	return &xhrMultipartSocket{t: t}
}

// Implements the socket interface for xhr-multipart transports.
type xhrMultipartSocket struct {
	t         *xhrMultipartTransport
	rwc       io.ReadWriteCloser
	connected bool
}

// String returns a verbose representation of the socket.
func (s *xhrMultipartSocket) String() string {
	return s.t.Resource()
}

// Transport returns the transport the socket is based on.
func (s *xhrMultipartSocket) Transport() Transport {
	return s.t
}

// Accepts a http connection & request pair. It hijacks the connection, sends headers and calls
// proceed if succesfull.
func (s *xhrMultipartSocket) accept(w http.ResponseWriter, req *http.Request, proceed func()) (err error) {
	if s.connected {
		return ErrConnected
	}

	rwc, _, err := w.(http.Hijacker).Hijack()

	if err == nil {
		rwc.(*net.TCPConn).SetReadTimeout(s.t.rtimeout)
		rwc.(*net.TCPConn).SetWriteTimeout(s.t.wtimeout)

		buf := new(bytes.Buffer)
		buf.WriteString("HTTP/1.0 200 OK\r\n")
		buf.WriteString("Content-Type: multipart/x-mixed-replace; boundary=\"socketio\"\r\n")
		buf.WriteString("Connection: keep-alive\r\n")
		if origin := req.Header.Get("Origin"); origin != "" {
			fmt.Fprintf(buf,
				"Access-Control-Allow-Origin: %s\r\nAccess-Control-Allow-Credentials: true\r\n",
				origin)
		}
		buf.WriteString("\r\n--socketio\r\n")

		if _, err = buf.WriteTo(rwc); err != nil {
			rwc.Close()
			return
		}

		s.rwc = rwc
		s.connected = true
		proceed()
	}

	return
}

func (s *xhrMultipartSocket) Read(p []byte) (n int, err error) {
	if !s.connected {
		return 0, ErrNotConnected
	}

	return s.rwc.Read(p)
}

// Write sends a single multipart message to the wire.
func (s *xhrMultipartSocket) Write(p []byte) (n int, err error) {
	if !s.connected {
		return 0, ErrNotConnected
	}

	return fmt.Fprintf(s.rwc, "Content-Type: text/plain\r\n\r\n%s\n--socketio\n", p)
}

func (s *xhrMultipartSocket) Close() error {
	if !s.connected {
		return ErrNotConnected
	}

	s.connected = false
	return s.rwc.Close()
}
