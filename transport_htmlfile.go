package socketio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
)

var HTMLFile = &Transport{
	Name:   "htmlfile",
	Type:   StreamingTransport,
	Hijack: htmlFileHijack,
}

var htmlfileHeader = "<html><body><script>var _ = function (msg) { parent.s._(msg, document); };</script>" + string(bytes.Repeat([]byte(" "), 174))

// Accepts a http connection & request pair. It hijacks the connection and calls
// proceed if succesfull.
func htmlFileHijack(w http.ResponseWriter, req *http.Request, proceed func(Socket)) error {
	rwc, _, err := w.(http.Hijacker).Hijack()
	if err == nil {
		conn := rwc.(*net.TCPConn)

		var buf bytes.Buffer
		buf.WriteString("HTTP/1.1 200 OK\r\n")
		buf.WriteString("Content-Type: text/html\r\n")
		buf.WriteString("Transfer-Encoding: chunked\r\n\r\n")

		fmt.Fprintf(&buf, "%x\r\n%s\r\n", len(htmlfileHeader), htmlfileHeader)

		if _, err = buf.WriteTo(conn); err == nil {
			proceed(&htmlFileSocket{conn, &buf, make([]byte, 1)})
		}
	}
	return err
}

type htmlFileSocket struct {
	net.Conn
	buf *bytes.Buffer
	rb  []byte
}

// Write sends a single message to the wire and closes the connection.
func (s *htmlFileSocket) Write(p []byte) (int, error) {
	s.buf.Reset()
	s.buf.WriteString("<script>_(")
	enc := json.NewEncoder(s.buf)
	if err := enc.Encode(string(p)); err != nil {
		return 0, err
	}
	s.buf.WriteString(");</script>")

	return fmt.Fprintf(s.Conn, "%x\r\n%s\r\n", s.buf.Len(), s.buf.Bytes())
}

func (s *htmlFileSocket) Receive(p *[]byte) (err error) {
	_, err = s.Read(s.rb)
	*p = s.rb
	return
}
