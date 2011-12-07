package socketio

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"unicode/utf8"
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

var (
	sioFrameDelim          = []byte("~m~")
	sioFrameDelimJSON      = []byte("~j~")
	sioFrameDelimHeartbeat = []byte("~h~")
)

// SioMessage fulfills the message interface.
type sioMessage struct {
	annotations map[string]string
	typ         uint8
	data        []byte
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
		if n, err := strconv.Atoi(string(sm.data)); err == nil {
			return heartbeat(n), true
		}
	}

	return -1, false
}

// Data returns the raw message as a string.
func (sm *sioMessage) Data() string {
	return string(sm.data)
}

// Bytes returns the raw message.
func (sm *sioMessage) Bytes() []byte {
	return sm.data
}

// JSON returns the JSON embedded in the message, if available.
func (sm *sioMessage) JSON() ([]byte, bool) {
	if sm.Type() == MessageJSON {
		return sm.data, true
	}

	return nil, false
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
func (enc *sioEncoder) Encode(dst io.Writer, payload interface{}) (err error) {
	enc.elem.Reset()

	switch t := payload.(type) {
	case heartbeat:
		s := strconv.Itoa(int(t))
		_, err = fmt.Fprintf(dst, "%s%d%s%s%s", sioFrameDelim, len(s)+len(sioFrameDelimHeartbeat), sioFrameDelim, sioFrameDelimHeartbeat, s)

	case handshake:
		_, err = fmt.Fprintf(dst, "%s%d%s%s", sioFrameDelim, len(t), sioFrameDelim, t)

	case []byte:
		l := utf8.RuneCount(t)
		if l == 0 {
			break
		}
		_, err = fmt.Fprintf(dst, "%s%d%s%s", sioFrameDelim, l, sioFrameDelim, t)

	case string:
		l := utf8.RuneCountInString(t)
		if l == 0 {
			break
		}
		_, err = fmt.Fprintf(dst, "%s%d%s%s", sioFrameDelim, l, sioFrameDelim, t)

	case int:
		s := strconv.Itoa(t)
		if s == "" {
			break
		}
		_, err = fmt.Fprintf(dst, "%s%d%s%s", sioFrameDelim, len(s), sioFrameDelim, s)

	default:
		data, err := json.Marshal(payload)
		if len(data) == 0 || err != nil {
			break
		}
		err = json.Compact(&enc.elem, data)
		if err != nil {
			break
		}

		_, err = fmt.Fprintf(dst, "%s%d%s%s", sioFrameDelim, utf8.RuneCount(enc.elem.Bytes())+len(sioFrameDelimJSON), sioFrameDelim, sioFrameDelimJSON)
		if err == nil {
			_, err = enc.elem.WriteTo(dst)
		}
	}

	return err
}

const (
	sioDecodeStateBegin = iota
	sioDecodeStateHeaderBegin
	sioDecodeStateLength
	sioDecodeStateHeaderEnd
	sioDecodeStateData
	sioDecodeStateEnd
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

func (dec *sioDecoder) Decode() (messages []Message, err error) {
	messages = make([]Message, 0, 1)
	var c int

L:
	for {
		if dec.state == sioDecodeStateBegin {
			dec.msg = &sioMessage{}
			dec.state = sioDecodeStateHeaderBegin
			dec.buf.Reset()
		}

		c, _, err = dec.src.ReadRune()
		if err != nil {
			break
		}

		switch dec.state {
		case sioDecodeStateHeaderBegin:
			dec.buf.WriteRune(c)
			if dec.buf.Len() == len(sioFrameDelim) {
				if !bytes.Equal(dec.buf.Bytes(), sioFrameDelim) {
					dec.Reset()
					return nil, errors.New("Malformed header")
				}

				dec.state = sioDecodeStateLength
				dec.buf.Reset()
			}
			continue

		case sioDecodeStateLength:
			if c >= '0' && c <= '9' {
				dec.buf.WriteRune(c)
				continue
			}

			if dec.length, err = strconv.Atoi(dec.buf.String()); err != nil {
				dec.Reset()
				return nil, err
			}

			dec.buf.Reset()
			dec.state = sioDecodeStateHeaderEnd
			fallthrough

		case sioDecodeStateHeaderEnd:
			dec.buf.WriteRune(c)
			if dec.buf.Len() < len(sioFrameDelim) {
				continue
			}

			if !bytes.Equal(dec.buf.Bytes(), sioFrameDelim) {
				dec.Reset()
				return nil, errors.New("Malformed header")
			}

			dec.state = sioDecodeStateData
			dec.buf.Reset()
			if dec.length > 0 {
				continue
			}
			fallthrough

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
				} else {
					break L
				}
			}

			data := dec.buf.Bytes()
			dec.msg.typ = sioMessageTypeMessage

			if bytes.HasPrefix(data, sioFrameDelimJSON) {
				dec.msg.annotations = make(map[string]string)
				dec.msg.annotations[SIOAnnotationJSON] = ""
				data = data[len(sioFrameDelimJSON):]
			} else if bytes.HasPrefix(data, sioFrameDelimHeartbeat) {
				dec.msg.typ = sioMessageTypeHeartbeat
				data = data[len(sioFrameDelimHeartbeat):]
			}
			dec.msg.data = make([]byte, len(data))
			copy(dec.msg.data, data)

			messages = append(messages, dec.msg)

			dec.state = sioDecodeStateBegin
			dec.buf.Reset()
			continue
		}

		dec.buf.WriteRune(c)
	}

	if err == io.EOF {
		err = nil
	}

	return
}
