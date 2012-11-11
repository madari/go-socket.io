package socketio

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// The different message types that are available.
const (
	MessageDisconnect = iota
	MessageConnect
	MessageHeartbeat
	MessageText
	MessageJSON
	MessageEvent
	MessageACK
	MessageError
	MessageNOOP
)

type disconnect string

type connect string

type heartbeat byte

type event struct {
	Args     []interface{} `json:"args,omitempty"`
	Name     string        `json:"name"`
	ack      bool
	endpoint string
	id       int
	raw      []byte
}

type ack struct {
	data  interface{}
	event bool
	id    int
}

type error_ struct {
	advice   int
	endpoint string
	reason   int
}

type noop byte

// Message wraps heartbeat, messageType and data methods.
//
// Heartbeat returns the heartbeat value encapsulated in the message and an true
// or if the message does not encapsulate a heartbeat a false is returned.
// MessageType returns messageText, messageHeartbeat or messageJSON.
// Data returns the raw (full) message received.
type Message struct {
	ack      bool
	data     []byte
	endpoint string
	event    *event
	id       int
	typ      uint8
}

func (m *Message) Event() (string, error) {
	if m.typ == MessageEvent {
		if m.event == nil {
			m.event = &event{}
		}
		if err := json.Unmarshal(m.data, m.event); err != nil {
			return "", err
		}
		return m.event.Name, nil
	}
	return "", errors.New("not an event")
}

func (m *Message) ReadArguments(a ...interface{}) error {
	if m.typ == MessageEvent {
		if m.event == nil {
			m.event = &event{}
		}
		m.event.Args = a
		if err := json.Unmarshal(m.data, m.event); err != nil {
			return err
		}
		return nil
	}
	return errors.New("not an event")
}

func (m *Message) Bytes() []byte {
	return m.data
}

func (m *Message) String() string {
	return string(m.data)
}

func (m *Message) Type() uint8 {
	return m.typ
}

func (m *Message) Inspect() string {
	buf := &bytes.Buffer{}
	buf.WriteString("{type: ")
	switch m.typ {
	case 0:
		fmt.Fprintf(buf, "disconnect, endpoint: %s", m.endpoint)
	case 1:
		fmt.Fprintf(buf, "connect, endpoint: %s", m.endpoint)
	case 2:
		fmt.Fprintf(buf, "heartbeat")
	case 3:
		fmt.Fprintf(buf, "message, id: %d, ack: %t, endpoint: %q, data: %q", m.id, m.ack, m.endpoint, m.data)
	case 4:
		fmt.Fprintf(buf, "json message, id: %d, ack: %t, endpoint: %q, data: %q", m.id, m.ack, m.endpoint, m.data)
	case 5:
		fmt.Fprintf(buf, "event, id: %d, ack: %t, endpoint: %q, data: %q", m.id, m.ack, m.endpoint, m.data)
	case 6:
		fmt.Fprintf(buf, "ack, data: %q", m.data)
	case 7:
		fmt.Fprintf(buf, "error, endpoint: %q, data: %q", m.endpoint, m.data)
	case 8:
		fmt.Fprintf(buf, "noop")
	default:
		fmt.Fprintf(buf, "[unknown]%+v", m)
	}
	buf.WriteString("}")
	return buf.String()
}

func (m *Message) zero() {
	m.ack = false
	m.data = nil
	m.endpoint = ""
	m.event = nil
	m.id = 0
	m.typ = 0
}
