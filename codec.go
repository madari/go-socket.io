package socketio

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
)

type Encoder struct {
	bytes.Buffer
	MustFrame bool
}

func (enc *Encoder) Encode(dst io.Writer, payload []interface{}) (err error) {
	if enc.MustFrame || len(payload) > 1 {
		for _, p := range payload {
			enc.Reset()
			if err = encodePacket(&enc.Buffer, p); err != nil {
				return
			}
			if _, err = fmt.Fprintf(dst, "\uFFFD%d\uFFFD%s", enc.Len(), enc.Bytes()); err != nil {
				return
			}
		}
	} else {
		enc.Reset()
		if err = encodePacket(&enc.Buffer, payload[0]); err == nil {
			_, err = enc.WriteTo(dst)
		}
	}
	return err
}

type Decoder struct {
	bytes.Buffer
}

func (dec *Decoder) Decode(msg *Message) error {
	r, rsize, err := dec.ReadRune()
	if err != nil {
		return err
	}

	if r == 0xFFFD && rsize == 3 {
		// the payload has multiple messages,
		// let's consume the first.
		// read frame length
		data := dec.Bytes()
		index := bytes.IndexRune(data, 0xFFFD)
		if index <= 0 {
			return errors.New("frame length is empty or missing")
		}
		length := positiveInt(data[:index])
		if length < 0 {
			return errors.New("frame length is not a positive integer")
		}
		data = data[index+rsize:]
		if len(data) < length {
			return errors.New("frame length is overflowing")
		}
		dec.Next(index + rsize + length)
		return decodePacket(data[:length], msg)
	}

	// rollback and read single entirely
	dec.UnreadRune()

	data := dec.Bytes()
	dec.Reset()
	return decodePacket(data, msg)
	return err
}

func decodePacket(data []byte, msg *Message) (err error) {
	msg.zero()

	// [message type] ':' [message id ('+')] ':' [message endpoint] (':' [message data]) 
	parts := bytes.SplitN(data, []byte{':'}, 4)
	if len(parts) < 3 {
		return errors.New("invalid number of parts")
	}

	// [message type]
	if i := positiveInt(parts[0]); i >= 0 && i <= 9 {
		msg.typ = uint8(i)
	} else {
		return errors.New("invalid type: " + string(parts[0]))
	}

	// [message id ('+')]
	l := len(parts[1])
	if l > 0 {
		if parts[1][l-1] == '+' {
			msg.ack = true
			parts[1] = parts[1][:l-1]
		}
		msg.id = positiveInt(parts[1])
	}

	// [message endpoint]
	msg.endpoint = string(parts[2])

	// [message data]
	if len(parts) == 4 {
		msg.data = parts[3]
	}

	return
}

func positiveInt(p []byte) (v int) {
	if len(p) < 1 {
		return -1
	}
	for _, c := range p {
		if c < '0' || c > '9' {
			return -1
		}
		v = v*10 + int(c) - '0'
	}
	return
}

func encodePacket(dst *bytes.Buffer, packet interface{}) (err error) {
	var msg *Message

	switch t := packet.(type) {
	case disconnect:
		msg = &Message{typ: MessageDisconnect, endpoint: string(t)}

	case connect:
		msg = &Message{typ: MessageConnect, endpoint: string(t)}

	case heartbeat:
		msg = &Message{typ: MessageHeartbeat}

	case *event:
		msg = &Message{typ: MessageEvent, id: t.id, ack: t.ack, endpoint: t.endpoint}
		if msg.data, err = json.Marshal(t); err != nil {
			return
		}

	case *ack:
		msg = &Message{typ: MessageACK}
		if t.event {
			if t.data != nil {
				var args []byte
				if args, err = json.Marshal(t.data); err != nil {
					return
				}
				msg.data = []byte(fmt.Sprintf("%d+%s", t.id, args))
			} else {
				msg.data = []byte(fmt.Sprintf("%d+", t.id))
			}
		} else {
			msg.data = []byte(fmt.Sprintf("%d", t.id))
		}

	case *error_:
		msg = &Message{typ: MessageError, endpoint: t.endpoint}
		if t.reason >= 0 {
			if t.advice >= 0 {
				msg.data = []byte(fmt.Sprintf("%d+%d", t.reason, t.advice))
			} else {
				msg.data = []byte(fmt.Sprintf("%d", t.reason))
			}
		} else if t.advice >= 0 {
			msg.data = []byte(fmt.Sprintf("+%d", t.advice))
		}

	case noop:
		msg = &Message{typ: MessageNOOP}

	case *Message:
		msg = t

	case int:
		s := strconv.Itoa(t)
		msg = &Message{typ: MessageText, data: []byte(s)}

	case string:
		msg = &Message{typ: MessageText, data: []byte(t)}

	case []byte:
		msg = &Message{typ: MessageText, data: t}

	default:
		msg = &Message{typ: MessageJSON}
		if msg.data, err = json.Marshal(t); err != nil {
			return
		}
	}

	if _, err = fmt.Fprintf(dst, "%d:", msg.typ); err != nil {
		return
	}
	if msg.id > 0 {
		if _, err = fmt.Fprintf(dst, "%d", msg.id); err != nil {
			return
		}
	}
	if msg.ack {
		if err = dst.WriteByte('+'); err != nil {
			return
		}
	}
	if err = dst.WriteByte(':'); err != nil {
		return
	}
	if _, err = dst.WriteString(msg.endpoint); err != nil {
		return
	}
	if msg.data != nil && len(msg.data) > 0 {
		if err = dst.WriteByte(':'); err != nil {
			return
		}
		_, err = dst.Write(msg.data)
	}
	return
}
