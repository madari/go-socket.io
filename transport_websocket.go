package socketio

import (
	"code.google.com/p/go.net/websocket"
	"errors"
	"net/http"
)

var ErrWebsocketHandshake = errors.New("websocket handshake error")

var Websocket = &Transport{
	Name:   "websocket",
	Type:   StreamingTransport,
	Hijack: websocketHijack,
}

func websocketHijack(w http.ResponseWriter, req *http.Request, proceed func(Socket)) (err error) {
	f := func(ws *websocket.Conn) {
		err = nil
		proceed(&websocketSocket{ws, nil})
	}

	err = ErrWebsocketHandshake
	websocket.Handler(f).ServeHTTP(w, req)
	return
}

type websocketSocket struct {
	*websocket.Conn
	rb []byte
}

func (s *websocketSocket) Receive(p *[]byte) (err error) {
	err = websocket.Message.Receive(s.Conn, &s.rb)
	*p = s.rb
	return
}

func (s *websocketSocket) Write(p []byte) (n int, err error) {
	err = websocket.Message.Send(s.Conn, string(p))
	if err == nil {
		n = len(p)
	}
	return
}
