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

type sioEncoder struct {
	elem bytes.Buffer
}

func (sc SIOCodec) NewEncoder() Encoder {
	return &sioEncoder{}
}

// Encode takes payload, encodes it and writes it to dst. Payload must be one
// of the following: a heartbeat, a handshake, []byte, string, int or anything
// than can be marshalled by the default json package. If payload can't be
// encoded or the writing fails, an error will be returned.
func (enc *sioEncoder) Encode(dst io.Writer, payload interface{}) (err os.Error) {
	enc.elem.Reset()

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
		err = json.Compact(&enc.elem, data)
		if err != nil {
			break
		}

		_, err = fmt.Fprintf(dst, "%d:%d:%s\n:", sioMessageTypeMessage, 2+len(SIOAnnotationJSON)+utf8.RuneCount(enc.elem.Bytes()), SIOAnnotationJSON)
		if err == nil {
			_, err = enc.elem.WriteTo(dst)
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


type sioDecoder struct {
	src           *bytes.Buffer
	buf           bytes.Buffer
	msg           *sioMessage
	key, value    string
	length, state int
}

func (sc SIOCodec) NewDecoder(src *bytes.Buffer) Decoder {
	return &sioDecoder{
		src:   src,
		state: sioDecodeStateBegin,
	}
}

func (dec *sioDecoder) Reset() {
	dec.buf.Reset()
	dec.src.Reset()
	dec.msg = nil
	dec.state = sioDecodeStateBegin
	dec.key = ""
	dec.value = ""
	dec.length = 0
}

func (dec *sioDecoder) Decode() (messages []Message, err os.Error) {
	var vec vector.Vector
	var c int
	var typ uint

L:
	for {
		c, _, err = dec.src.ReadRune()
		if err != nil {
			break
		}

		if dec.state == sioDecodeStateBegin {
			dec.msg = &sioMessage{}
			dec.state = sioDecodeStateType
			dec.buf.Reset()
		}

		switch dec.state {
		case sioDecodeStateType:
			if c == ':' {
				if typ, err = strconv.Atoui(dec.buf.String()); err != nil {
					dec.Reset()
					return nil, err
				}
				dec.msg.typ = uint8(typ)
				dec.buf.Reset()
				dec.state = sioDecodeStateLength
				continue
			}

		case sioDecodeStateLength:
			if c == ':' {
				if dec.length, err = strconv.Atoi(dec.buf.String()); err != nil {
					dec.Reset()
					return nil, err
				}
				dec.buf.Reset()

				switch dec.msg.typ {
				case sioMessageTypeMessage:
					dec.state = sioDecodeStateAnnotationKey

				case sioMessageTypeDisconnect:
					dec.state = sioDecodeStateTrailer

				default:
					dec.state = sioDecodeStateData
				}

				continue
			}

		case sioDecodeStateAnnotationKey:
			dec.length--

			switch c {
			case ':':
				if dec.buf.Len() == 0 {
					dec.state = sioDecodeStateData
				} else {
					dec.key = dec.buf.String()
					dec.buf.Reset()
					dec.state = sioDecodeStateAnnotationValue
				}

				continue

			case '\n':
				if dec.buf.Len() == 0 {
					dec.Reset()
					return nil, os.NewError("expecting key, but got...")
				}
				dec.key = dec.buf.String()
				if dec.msg.annotations == nil {
					dec.msg.annotations = make(map[string]string)
				}

				dec.msg.annotations[dec.key] = ""
				dec.buf.Reset()

				continue
			}

		case sioDecodeStateAnnotationValue:
			dec.length--

			if c == '\n' || c == ':' {
				dec.value = dec.buf.String()
				if dec.msg.annotations == nil {
					dec.msg.annotations = make(map[string]string)
				}

				dec.msg.annotations[dec.key] = dec.value
				dec.buf.Reset()

				if c == '\n' {
					dec.state = sioDecodeStateAnnotationKey
				} else {
					dec.state = sioDecodeStateData
				}
				continue
			}

		case sioDecodeStateData:
			if dec.length > 0 {
				dec.buf.WriteRune(c)
				dec.length--

				utf8str := utf8.NewString(dec.src.String())
				if utf8str.RuneCount() >= dec.length {
					str := utf8str.Slice(0, dec.length)
					dec.buf.WriteString(str)
					dec.src.Next(len(str))
					dec.length = 0
					continue
				} else {
					break L
				}
			}

			dec.msg.data = dec.buf.String()
			dec.buf.Reset()
			dec.state = sioDecodeStateTrailer
			fallthrough

		case sioDecodeStateTrailer:
			if c == ',' {
				vec.Push(dec.msg)
				dec.state = sioDecodeStateBegin
				continue
			} else {
				dec.Reset()
				return nil, os.NewError("Expecting trailer but got... " + string(c))
			}
		}

		dec.buf.WriteRune(c)
	}

	messages = make([]Message, vec.Len())
	for i, v := range vec {
		messages[i] = v.(*sioMessage)
	}

	if err == os.EOF {
		err = nil
	}

	return
}
