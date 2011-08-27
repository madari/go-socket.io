package socketio

import (
	"bytes"
	"fmt"
	"io"
	"json"
	"os"
	"strconv"
	"utf8"
)

// SIOStreamingCodec is the codec used by the official Socket.IO client by LearnBoost
// under the development branch. This will be the default codec for 0.7 release.
type SIOStreamingCodec struct{}

type sioStreamingEncoder struct {
	elem bytes.Buffer
}

func (sc SIOStreamingCodec) NewEncoder() Encoder {
	return &sioStreamingEncoder{}
}

// Encode takes payload, encodes it and writes it to dst. Payload must be one
// of the following: a heartbeat, a handshake, []byte, string, int or anything
// than can be marshalled by the default json package. If payload can't be
// encoded or the writing fails, an error will be returned.
func (enc *sioStreamingEncoder) Encode(dst io.Writer, payload interface{}) (err os.Error) {
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
	sioStreamingDecodeStateBegin = iota
	sioStreamingDecodeStateType
	sioStreamingDecodeStateLength
	sioStreamingDecodeStateAnnotationKey
	sioStreamingDecodeStateAnnotationValue
	sioStreamingDecodeStateData
	sioStreamingDecodeStateTrailer
)

type sioStreamingDecoder struct {
	src           *bytes.Buffer
	buf           bytes.Buffer
	msg           *sioMessage
	key, value    string
	length, state int
}

func (sc SIOStreamingCodec) NewDecoder(src *bytes.Buffer) Decoder {
	return &sioStreamingDecoder{
		src:   src,
		state: sioStreamingDecodeStateBegin,
	}
}

func (dec *sioStreamingDecoder) Reset() {
	dec.buf.Reset()
	dec.src.Reset()
	dec.msg = nil
	dec.state = sioStreamingDecodeStateBegin
	dec.key = ""
	dec.value = ""
	dec.length = 0
}

func (dec *sioStreamingDecoder) Decode() (messages []Message, err os.Error) {
	messages = make([]Message, 0, 1)
	var c int
	var typ uint

L:
	for {
		c, _, err = dec.src.ReadRune()
		if err != nil {
			break
		}

		if dec.state == sioStreamingDecodeStateBegin {
			dec.msg = &sioMessage{}
			dec.state = sioStreamingDecodeStateType
			dec.buf.Reset()
		}

		switch dec.state {
		case sioStreamingDecodeStateType:
			if c == ':' {
				if typ, err = strconv.Atoui(dec.buf.String()); err != nil {
					dec.Reset()
					return nil, err
				}
				dec.msg.typ = uint8(typ)
				dec.buf.Reset()
				dec.state = sioStreamingDecodeStateLength
				continue
			}

		case sioStreamingDecodeStateLength:
			if c == ':' {
				if dec.length, err = strconv.Atoi(dec.buf.String()); err != nil {
					dec.Reset()
					return nil, err
				}
				dec.buf.Reset()

				switch dec.msg.typ {
				case sioMessageTypeMessage:
					dec.state = sioStreamingDecodeStateAnnotationKey

				case sioMessageTypeDisconnect:
					dec.state = sioStreamingDecodeStateTrailer

				default:
					dec.state = sioStreamingDecodeStateData
				}

				continue
			}

		case sioStreamingDecodeStateAnnotationKey:
			dec.length--

			switch c {
			case ':':
				if dec.buf.Len() == 0 {
					dec.state = sioStreamingDecodeStateData
				} else {
					dec.key = dec.buf.String()
					dec.buf.Reset()
					dec.state = sioStreamingDecodeStateAnnotationValue
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

		case sioStreamingDecodeStateAnnotationValue:
			dec.length--

			if c == '\n' || c == ':' {
				dec.value = dec.buf.String()
				if dec.msg.annotations == nil {
					dec.msg.annotations = make(map[string]string)
				}

				dec.msg.annotations[dec.key] = dec.value
				dec.buf.Reset()

				if c == '\n' {
					dec.state = sioStreamingDecodeStateAnnotationKey
				} else {
					dec.state = sioStreamingDecodeStateData
				}
				continue
			}

		case sioStreamingDecodeStateData:
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

			data := dec.buf.Bytes()
			dec.msg.data = make([]byte, len(data))
			copy(dec.msg.data, data)

			dec.buf.Reset()
			dec.state = sioStreamingDecodeStateTrailer
			fallthrough

		case sioStreamingDecodeStateTrailer:
			if c == ',' {
				messages = append(messages, dec.msg)
				dec.state = sioStreamingDecodeStateBegin
				continue
			} else {
				dec.Reset()
				return nil, os.NewError("Expecting trailer but got... " + string(c))
			}
		}

		dec.buf.WriteRune(c)
	}

	if err == os.EOF {
		err = nil
	}

	return
}
