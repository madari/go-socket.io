package socketio

import (
	"websocket"
	"io"
	"os"
	"strconv"
)

// Client is a toy interface.
type Client interface {
	io.Closer

	Dial(string, string) os.Error
	Send(interface{}) os.Error
	OnDisconnect(func())
	OnMessage(func(Message))
	SessionID() SessionID
}

// WebsocketClient is a toy that implements the Client interface.
type WebsocketClient struct {
	connected    bool
	codec        Codec
	sessionid    SessionID
	ws           *websocket.Conn
	onDisconnect func()
	onMessage    func(Message)
}

func NewWebsocketClient(codec Codec) Client {
	return &WebsocketClient{codec: codec}
}

func (wc *WebsocketClient) Dial(rawurl string, origin string) (err os.Error) {
	var buf []byte
	var nr int
	var messages []Message

	if wc.connected {
		return ErrConnected
	}

	if wc.ws, err = websocket.Dial(rawurl, "", origin); err != nil {
		return
	}

	// read handshake
	buf = make([]byte, 2048)
	if nr, err = wc.ws.Read(buf); err != nil {
		wc.ws.Close()
		return os.NewError("Dial: " + err.String())
	}

	if messages, err = wc.codec.Decode(buf[0:nr]); err != nil {
		wc.ws.Close()
		return os.NewError("Dial: " + err.String())
	}

	if len(messages) != 1 {
		wc.ws.Close()
		return os.NewError("Dial: expected exactly 1 message, but got " + strconv.Itoa(len(messages)))
	}

	if messages[0].Type() != MessageHandshake {
		wc.ws.Close()
		return os.NewError("Dial: expected handshake, but got " + messages[0].Data())
	}

	wc.sessionid = SessionID(messages[0].Data())
	if wc.sessionid == "" {
		wc.ws.Close()
		return os.NewError("Dial: received empty sessionid")
	}

	wc.connected = true

	go wc.reader()
	return
}

func (wc *WebsocketClient) SessionID() SessionID {
	return wc.sessionid
}

func (wc *WebsocketClient) reader() {
	var nr int
	var err os.Error
	var messages []Message

	defer wc.Close()

	buf := make([]byte, 2048)
	for {
		if nr, err = wc.ws.Read(buf); err != nil {
			return
		}

		if nr > 0 {
			if messages, err = wc.codec.Decode(buf[0:nr]); err != nil {
				return
			}

			for _, msg := range messages {
				if hb, ok := msg.heartbeat(); ok {
					if err = wc.Send(heartbeat(hb)); err != nil {
						return
					}
				} else if wc.onMessage != nil {
					wc.onMessage(msg)
				}
			}
		}
	}
}

func (wc *WebsocketClient) OnDisconnect(f func()) {
	wc.onDisconnect = f
}
func (wc *WebsocketClient) OnMessage(f func(Message)) {
	wc.onMessage = f
}

func (wc *WebsocketClient) Send(payload interface{}) os.Error {
	if wc.ws == nil {
		return ErrNotConnected
	}

	return wc.codec.Encode(wc.ws, payload)
}

func (wc *WebsocketClient) Close() os.Error {
	if !wc.connected {
		return ErrNotConnected
	}
	wc.connected = false

	if wc.onDisconnect != nil {
		wc.onDisconnect()
	}

	return wc.ws.Close()
}
