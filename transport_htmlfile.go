package socketio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

var htmlfileHeader = "<html><body>" + strings.Repeat(" ", 244)

// The xhr-multipart transport.
type htmlfileTransport struct {
	rtimeout int64 // The period during which the client must send a message.
	wtimeout int64 // The period during which a write must succeed.
}

// Creates a new xhr-multipart transport with the given read and write timeouts.
func NewHTMLFileTransport(rtimeout, wtimeout int64) Transport {
	return &htmlfileTransport{rtimeout, wtimeout}
}

// Returns the resource name.
func (t *htmlfileTransport) Resource() string {
	return "htmlfile"
}

// Creates a new socket that can be used with a connection.
func (t *htmlfileTransport) newSocket() socket {
	return &htmlfileSocket{t: t}
}

// Implements the socket interface for xhr-multipart transports.
type htmlfileSocket struct {
	t         *htmlfileTransport
	rwc       io.ReadWriteCloser
	connected bool
}

// String returns a verbose representation of the socket.
func (s *htmlfileSocket) String() string {
	return s.t.Resource()
}

// Transport returns the transport the socket is based on.
func (s *htmlfileSocket) Transport() Transport {
	return s.t
}

// Accepts a http connection & request pair. It hijacks the connection, sends headers and calls
// proceed if succesfull.
func (s *htmlfileSocket) accept(w http.ResponseWriter, req *http.Request, proceed func()) (err error) {
	if s.connected {
		return ErrConnected
	}

	rwc, _, err := w.(http.Hijacker).Hijack()

	if err == nil {
		rwc.(*net.TCPConn).SetReadTimeout(s.t.rtimeout)
		rwc.(*net.TCPConn).SetWriteTimeout(s.t.wtimeout)

		buf := new(bytes.Buffer)
		buf.WriteString("HTTP/1.1 200 OK\r\n")
		buf.WriteString("Content-Type: text/html\r\n")
		buf.WriteString("Connection: keep-alive\r\n")
		buf.WriteString("Transfer-Encoding: chunked\r\n\r\n")
		if _, err = buf.WriteTo(rwc); err != nil {
			rwc.Close()
			return
		}
		if _, err = fmt.Fprintf(rwc, "%x\r\n%s\r\n", len(htmlfileHeader), htmlfileHeader); err != nil {
			rwc.Close()
			return
		}

		s.rwc = rwc
		s.connected = true
		proceed()
	}

	return
}

func (s *htmlfileSocket) Read(p []byte) (n int, err error) {
	if !s.connected {
		return 0, ErrNotConnected
	}

	return s.rwc.Read(p)
}

// Write sends a single multipart message to the wire.
func (s *htmlfileSocket) Write(p []byte) (n int, err error) {
	if !s.connected {
		return 0, ErrNotConnected
	}

	var jp []byte
	var buf bytes.Buffer
	if jp, err = json.Marshal(string(p)); err != nil {
		return
	}

	fmt.Fprintf(&buf, "<script>parent.s._(%s, document);</script>", jp)
	return fmt.Fprintf(s.rwc, "%x\r\n%s\r\n", buf.Len(), buf.String())
}

func (s *htmlfileSocket) Close() error {
	if !s.connected {
		return ErrNotConnected
	}

	s.connected = false
	return s.rwc.Close()
}
