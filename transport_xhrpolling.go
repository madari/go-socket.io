package socketio

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
)

var XHRPolling = &Transport{
	Name:   "xhr-polling",
	Type:   PollingTransport,
	Hijack: xhrPollingHijack,
}

// Accepts a http connection & request pair. It hijacks the connection and calls
// proceed if succesfull.
func xhrPollingHijack(w http.ResponseWriter, req *http.Request, proceed func(Socket)) error {
	rwc, _, err := w.(http.Hijacker).Hijack()
	if err == nil {
		conn := rwc.(*net.TCPConn)

		var buf bytes.Buffer
		buf.WriteString("HTTP/1.0 200 OK\r\n")
		buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		if origin := req.Header.Get("Origin"); origin != "" {
			buf.WriteString("Access-Control-Allow-Origin: ")
			buf.WriteString(origin)
			buf.WriteString("\r\nAccess-Control-Allow-Credentials: true\r\n")
		}

		if _, err = buf.WriteTo(conn); err == nil {
			proceed(&xhrPollingSocket{conn, make([]byte, 1)})
		}
	}
	return err
}

type xhrPollingSocket struct {
	net.Conn
	rb []byte
}

// Write sends a single message to the wire and closes the connection.
func (s *xhrPollingSocket) Write(p []byte) (int, error) {
	defer s.Close()
	return fmt.Fprintf(s.Conn, "Content-Length: %d\r\n\r\n%s", len(p), p)
}

func (s *xhrPollingSocket) Receive(p *[]byte) (err error) {
	_, err = s.Read(s.rb)
	*p = s.rb
	return
}
