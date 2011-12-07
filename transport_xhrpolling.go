package socketio

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
)

// The xhr-polling transport.
type xhrPollingTransport struct {
	rtimeout int64 // The period during which the client must send a message.
	wtimeout int64 // The period during which a write must succeed.
}

// Creates a new xhr-polling transport with the given read and write timeouts.
func NewXHRPollingTransport(rtimeout, wtimeout int64) Transport {
	return &xhrPollingTransport{rtimeout, wtimeout}
}

// Returns the resource name.
func (t *xhrPollingTransport) Resource() string {
	return "xhr-polling"
}

// Creates a new socket that can be used with a connection.
func (t *xhrPollingTransport) newSocket() socket {
	return &xhrPollingSocket{t: t}
}

// Implements the socket interface for xhr-polling transports.
type xhrPollingSocket struct {
	t         *xhrPollingTransport
	rwc       io.ReadWriteCloser
	req       *http.Request
	connected bool
}

// String returns the verbose representation of the socket.
func (s *xhrPollingSocket) String() string {
	return s.t.Resource()
}

// Transport returns the transport the socket is based on.
func (s *xhrPollingSocket) Transport() Transport {
	return s.t
}

// Accepts a http connection & request pair. It hijacks the connection and calls
// proceed if succesfull.
func (s *xhrPollingSocket) accept(w http.ResponseWriter, req *http.Request, proceed func()) (err error) {
	if s.connected {
		return ErrConnected
	}

	s.req = req
	s.rwc, _, err = w.(http.Hijacker).Hijack()
	if err == nil {
		s.rwc.(*net.TCPConn).SetReadTimeout(s.t.rtimeout)
		s.rwc.(*net.TCPConn).SetWriteTimeout(s.t.wtimeout)
		s.connected = true
		proceed()
	}
	return
}

func (s *xhrPollingSocket) Read(p []byte) (int, error) {
	if !s.connected {
		return 0, ErrNotConnected
	}

	return s.rwc.Read(p)
}

// Write sends a single message to the wire and closes the connection.
func (s *xhrPollingSocket) Write(p []byte) (int, error) {
	if !s.connected {
		return 0, ErrNotConnected
	}

	defer s.Close()

	buf := new(bytes.Buffer)

	buf.WriteString("HTTP/1.0 200 OK\r\n")
	buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	fmt.Fprintf(buf, "Content-Length: %d\r\n", len(p))

	if origin := s.req.Header.Get("Origin"); origin != "" {
		fmt.Fprintf(buf, "Access-Control-Allow-Origin: %s\r\n", origin)
		buf.WriteString("Access-Control-Allow-Credentials: true\r\n")
	}

	buf.WriteString("\r\n")
	buf.Write(p)

	nr, err := buf.WriteTo(s.rwc)
	return int(nr), err
}

func (s *xhrPollingSocket) Close() error {
	if !s.connected {
		return ErrNotConnected
	}

	s.connected = false
	return s.rwc.Close()
}
