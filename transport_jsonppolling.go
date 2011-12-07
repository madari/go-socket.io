package socketio

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
)

// The jsonp-polling transport.
type jsonpPollingTransport struct {
	rtimeout int64 // The period during which the client must send a message.
	wtimeout int64 // The period during which a write must succeed.
}

// Creates a new json-polling transport with the given read and write timeouts.
func NewJSONPPollingTransport(rtimeout, wtimeout int64) Transport {
	return &jsonpPollingTransport{rtimeout, wtimeout}
}

// Returns the resource name.
func (t *jsonpPollingTransport) Resource() string {
	return "jsonp-polling"
}

// Creates a new socket which can be used be a connection.
func (t *jsonpPollingTransport) newSocket() (s socket) {
	return &jsonpPollingSocket{t: t}
}

// Implements the socket interface.
type jsonpPollingSocket struct {
	t         *jsonpPollingTransport
	rwc       io.ReadWriteCloser
	index     int
	connected bool
}

// String returns the verbose representation of the transport instance.
func (s *jsonpPollingSocket) String() string {
	return s.t.Resource()
}

// Transport return the transport the socket is based on.
func (s *jsonpPollingSocket) Transport() Transport {
	return s.t
}

// Accepts a http connection & request pair. It hijacks the connection and calls
// proceed if succesfull.
func (s *jsonpPollingSocket) accept(w http.ResponseWriter, req *http.Request, proceed func()) (err error) {
	if s.connected {
		return ErrConnected
	}

	rwc, _, err := w.(http.Hijacker).Hijack()
	if err == nil {
		rwc.(*net.TCPConn).SetReadTimeout(s.t.rtimeout)
		rwc.(*net.TCPConn).SetWriteTimeout(s.t.wtimeout)
		s.rwc = rwc
		s.connected = true
		s.index = 0
		if ts := req.FormValue("t"); ts != "" {
			if index, err := strconv.Atoi(ts); err == nil {
				s.index = index
			}
		}
		proceed()
	}
	return
}

func (s *jsonpPollingSocket) Read(p []byte) (n int, err error) {
	if !s.connected {
		return 0, ErrNotConnected
	}

	return s.rwc.Read(p)
}

// Write sends a single message to the wire and closes the connection.
func (s *jsonpPollingSocket) Write(p []byte) (n int, err error) {
	if !s.connected {
		return 0, ErrNotConnected
	}

	defer s.Close()

	var jp []byte
	if jp, err = json.Marshal(string(p)); err != nil {
		return
	}

	jsonp := fmt.Sprintf("io.JSONP[%d]._(%s);", s.index, string(jp))
	return fmt.Fprintf(s.rwc,
		"HTTP/1.0 200 OK\r\nContent-Type: text/javascript; charset=UTF-8\r\nContent-Length: %d\r\n\r\n%s",
		len(jsonp), jsonp)
}

func (s *jsonpPollingSocket) Close() error {
	if !s.connected {
		return ErrNotConnected
	}

	s.connected = false
	return s.rwc.Close()
}
