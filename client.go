package socketio

import (
	"bytes"
	"errors"
	"io"
	"strconv"
	"websocket"
)

// Client is a toy interface.
type Client interface {
	io.Closer

	Dial(string, string) error
	Send(interface{}) error
	OnDisconnect(func())
	OnMessage(func(Message))
	SessionID() SessionID
}

// WebsocketClient is a toy that implements the Client interface.
type WebsocketClient struct {
	connected    bool
	enc          Encoder
	dec          Decoder
	decBuf       bytes.Buffer
	codec        Codec
	sessionid    SessionID
	ws           *websocket.Conn
	onDisconnect func()
	onMessage    func(Message)
}

func NewWebsocketClient(codec Codec) (wc *WebsocketClient) {
	wc = &WebsocketClient{enc: codec.NewEncoder(), codec: codec}
	wc.dec = codec.NewDecoder(&wc.decBuf)
	return
}

func (wc *WebsocketClient) Dial(rawurl string, origin string) (err error) {
	var messages []Message
	var nr int

	if wc.connected {
		return ErrConnected
	}

	if wc.ws, err = websocket.Dial(rawurl, "", origin); err != nil {
		return
	}

	// read handshake
	buf := make([]byte, 2048)
	if nr, err = wc.ws.Read(buf); err != nil {
		wc.ws.Close()
		return errors.New("Dial: " + err.Error())
	}
	wc.decBuf.Write(buf[0:nr])

	if messages, err = wc.dec.Decode(); err != nil {
		wc.ws.Close()
		return errors.New("Dial: " + err.Error())
	}

	if len(messages) != 1 {
		wc.ws.Close()
		return errors.New("Dial: expected exactly 1 message, but got " + strconv.Itoa(len(messages)))
	}

	// TODO: Fix me: The original Socket.IO codec does not have a special encoding for handshake
	// so we should just assume that the first message is the handshake.
	// The old codec should be gone pretty soon (waiting for 0.7 release) so this might suffice
	// until then.
	if _, ok := wc.codec.(SIOCodec); !ok {
		if messages[0].Type() != MessageHandshake {
			wc.ws.Close()
			return errors.New("Dial: expected handshake, but got " + messages[0].Data())
		}
	}

	wc.sessionid = SessionID(messages[0].Data())
	if wc.sessionid == "" {
		wc.ws.Close()
		return errors.New("Dial: received empty sessionid")
	}

	wc.connected = true

	go wc.reader()
	return
}

func (wc *WebsocketClient) SessionID() SessionID {
	return wc.sessionid
}

func (wc *WebsocketClient) reader() {
	var err error
	var nr int
	var messages []Message
	buf := make([]byte, 2048)

	defer wc.Close()

	for {
		if nr, err = wc.ws.Read(buf); err != nil {
			return
		}
		if nr > 0 {
			wc.decBuf.Write(buf[0:nr])
			if messages, err = wc.dec.Decode(); err != nil {
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

func (wc *WebsocketClient) Send(payload interface{}) error {
	if wc.ws == nil {
		return ErrNotConnected
	}

	return wc.enc.Encode(wc.ws, payload)
}

func (wc *WebsocketClient) Close() error {
	if !wc.connected {
		return ErrNotConnected
	}
	wc.connected = false

	if wc.onDisconnect != nil {
		wc.onDisconnect()
	}

	return wc.ws.Close()
}
