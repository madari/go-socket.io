package socketio

import (
	"bytes"
	"container/vector"
	"fmt"
	"io"
	"json"
	"os"
	"strconv"
	"utf8"
)

// The various delimiters used for framing in the socket.io protocol.
const (
	SIOAnnotationRealm = "r"
	SIOAnnotationJSON  = "j"

	sioMessageTypeDisconnect = 0
	sioMessageTypeMessage    = 1
	sioMessageTypeHeartbeat  = 2
	sioMessageTypeHandshake  = 3
)

// SioMessage fulfills the message interface.
type sioMessage struct {
	annotations map[string]string
	typ         uint8
	data        string
}

// MessageType checks if the message starts with sioFrameDelimJSON or
// sioFrameDelimHeartbeat. If the prefix is something else, then the message
// is interpreted as a basic messageText.
func (sm *sioMessage) Type() uint8 {
	switch sm.typ {
	case sioMessageTypeMessage:
		if _, ok := sm.Annotation(SIOAnnotationJSON); ok {
			return MessageJSON
		}

	case sioMessageTypeDisconnect:
		return MessageDisconnect

	case sioMessageTypeHeartbeat:
		return MessageHeartbeat

	case sioMessageTypeHandshake:
		return MessageHandshake
	}

	return MessageText
}

func (sm *sioMessage) Annotations() map[string]string {
	return sm.annotations
}

func (sm *sioMessage) Annotation(key string) (value string, ok bool) {
	if sm.annotations == nil {
		return "", false
	}
	value, ok = sm.annotations[key]
	return
}

// Heartbeat looks for a heartbeat value in the message. If a such value
// can be extracted, then that value and a true is returned. Otherwise a
// false will be returned.
func (sm *sioMessage) heartbeat() (heartbeat, bool) {
	if sm.typ == sioMessageTypeHeartbeat {
		if n, err := strconv.Atoi(sm.data); err == nil {
			return heartbeat(n), true
		}
	}

	return -1, false
}

// Data returns the raw message.
func (sm *sioMessage) Data() string {
	return string(sm.data)
}

// JSON returns the JSON embedded in the message, if available.
func (sm *sioMessage) JSON() (string, bool) {
	if sm.Type() == MessageJSON {
		return sm.data, true
	}

	return "", false
}

// SIOCodec is the codec used by the official Socket.IO client by LearnBoost.
// Each message is framed with a prefix and goes like this:
// <DELIM>DATA-LENGTH<DELIM>[<OPTIONAL DELIM>]DATA.
type SIOCodec struct{}

// Encode takes payload, encodes it and writes it to dst. Payload must be one
// of the following: a heartbeat, a handshake, []byte, string, int or anything
// than can be marshalled by the default json package. If payload can't be
// encoded or the writing fails, an error will be returned.
func (c SIOCodec) Encode(dst io.Writer, payload interface{}) (err os.Error) {
	elem := new(bytes.Buffer)

	switch t := payload.(type) {
	case heartbeat:
		s := strconv.Itoa(int(t))
		_, err = fmt.Fprintf(dst, "%d:%d:%s,", sioMessageTypeHeartbeat, len(s), s)

	case handshake:
		_, err = fmt.Fprintf(dst, "%d:%d:%s,", sioMessageTypeHandshake, len(t), t)

	case []byte:
		l := utf8.RuneCount(t)
		if l == 0 {
			break
		}
		_, err = fmt.Fprintf(dst, "%d:%d::%s,", sioMessageTypeMessage, 1+l, t)

	case string:
		l := utf8.RuneCountInString(t)
		if l == 0 {
			break
		}
		_, err = fmt.Fprintf(dst, "%d:%d::%s,", sioMessageTypeMessage, 1+l, t)

	case int:
		s := strconv.Itoa(t)
		if s == "" {
			break
		}
		_, err = fmt.Fprintf(dst, "%d:%d::%s,", sioMessageTypeMessage, 1+len(s), s)

	default:
		data, err := json.Marshal(payload)
		if len(data) == 0 || err != nil {
			break
		}
		err = json.Compact(elem, data)
		if err != nil {
			break
		}

		_, err = fmt.Fprintf(dst, "%d:%d:%s\n:", sioMessageTypeMessage, 2+len(SIOAnnotationJSON)+utf8.RuneCount(elem.Bytes()), SIOAnnotationJSON)
		if err == nil {
			_, err = elem.WriteTo(dst)
			if err == nil {
				_, err = dst.Write([]byte{','})
			}
		}
	}

	return err
}

const (
	sioDecodeStateBegin = iota
	sioDecodeStateType
	sioDecodeStateLength
	sioDecodeStateAnnotationKey
	sioDecodeStateAnnotationValue
	sioDecodeStateData
	sioDecodeStateTrailer
)

func (c SIOCodec) Decode(payload []byte) (messages []Message, err os.Error) {
	var msg *sioMessage
	var key, value string
	var length, index int
	var vec vector.Vector
	var typ uint
	str := string(payload)
	state := sioDecodeStateBegin

L:
	for index < len(str) {
		c := str[index]

		switch state {
		case sioDecodeStateBegin:
			msg = &sioMessage{}
			state = sioDecodeStateType
			continue

		case sioDecodeStateType:
			if c == ':' {
				if typ, err = strconv.Atoui(str[0:index]); err != nil {
					return nil, err
				}
				msg.typ = uint8(typ)
				str = str[index+1:]
				index = 0
				state = sioDecodeStateLength
				continue
			}

		case sioDecodeStateLength:
			if c == ':' {
				if length, err = strconv.Atoi(str[0:index]); err != nil {
					return nil, err
				}
				str = str[index+1:]
				index = 0

				switch msg.typ {
				case sioMessageTypeDisconnect:
					state = sioDecodeStateTrailer

				case sioMessageTypeHeartbeat, sioMessageTypeHandshake:
					state = sioDecodeStateData

				default:
					state = sioDecodeStateAnnotationKey

				}
				continue
			}

		case sioDecodeStateAnnotationKey:
			length--

			switch c {
			case ':':
				if index == 0 {
					state = sioDecodeStateData
				} else {
					key = str[0:index]
					state = sioDecodeStateAnnotationValue
				}
				str = str[index+1:]
				index = 0
				continue

			case '\n':
				if index == 0 {
					return nil, os.NewError("expecting key, but got '" + str + "'")
				}
				key = str[0:index]
				if msg.annotations == nil {
					msg.annotations = make(map[string]string)
				}
				msg.annotations[key] = ""
				str = str[index+1:]
				index = 0
				continue
			}

		case sioDecodeStateAnnotationValue:
			length--

			switch c {
			case '\n':
				value = str[0:index]
				if msg.annotations == nil {
					msg.annotations = make(map[string]string)
				}
				msg.annotations[key] = value
				str = str[index+1:]
				index = 0

				state = sioDecodeStateAnnotationKey
				continue

			case ':':
				if index == 0 {
					value = ""
				} else {
					value = str[0:index]
				}

				if msg.annotations == nil {
					msg.annotations = make(map[string]string)
				}
				msg.annotations[key] = value
				str = str[index+1:]
				index = 0

				state = sioDecodeStateData
				continue
			}

		case sioDecodeStateData:
			utf8str := utf8.NewString(str)
			if length < 0 || length > utf8str.RuneCount() {
				return nil, os.NewError("bad data")
			}
			msg.data = utf8str.Slice(0, length)
			str = str[len(msg.data):]
			index = 0

			state = sioDecodeStateTrailer
			continue

		case sioDecodeStateTrailer:
			if c == ',' {
				vec.Push(msg)
				str = str[1:]
				index = 0

				state = sioDecodeStateBegin
				continue
			} else {
				return nil, os.NewError("Expecting trailer but got '" + str + "'")
			}
		}

		index++
	}

	if state != sioDecodeStateBegin {
		return nil, os.NewError("Expected sioDecodeStateBegin, but was something else: " + strconv.Itoa(state) + ". unscanned: '" + str + "'")
	}

	messages = make([]Message, vec.Len())
	for i, v := range vec {
		messages[i] = v.(*sioMessage)
	}

	return
}
