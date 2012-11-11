package socketio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
)

var JSONPPolling = &Transport{
	Name:        "jsonp-polling",
	Type:        PollingTransport,
	PostEncoded: true,
	Hijack:      jsonpPollingHijack,
}

func jsonpPollingHijack(w http.ResponseWriter, req *http.Request, proceed func(Socket)) error {
	rwc, _, err := w.(http.Hijacker).Hijack()
	if err == nil {
		conn := rwc.(*net.TCPConn)

		var buf bytes.Buffer
		buf.WriteString("HTTP/1.0 200 OK\r\n")
		buf.WriteString("Content-Type: text/javascript; charset=UTF-8\r\n")
		buf.WriteString("X-XSS-Protection: 0\r\n")

		i, _ := strconv.ParseUint(req.FormValue("i"), 10, 0)

		if _, err = buf.WriteTo(conn); err == nil {
			proceed(&jsonpPollingSocket{conn, i, make([]byte, 1)})
		}
	}
	return err
}

type jsonpPollingSocket struct {
	net.Conn
	i  uint64
	rb []byte
}

// Write sends a single message to the wire and closes the connection.
func (s *jsonpPollingSocket) Write(p []byte) (int, error) {
	defer s.Close()

	var buf bytes.Buffer

	fmt.Fprintf(&buf, "io.j[%d](", s.i)
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(string(p)); err != nil {
		return 0, err
	}
	buf.WriteString(");")

	return fmt.Fprintf(s.Conn, "Content-Length: %d\r\n\r\n%s", buf.Len(), buf.Bytes())
}

func (s *jsonpPollingSocket) Receive(p *[]byte) (err error) {
	_, err = s.Read(s.rb)
	*p = s.rb
	return
}
