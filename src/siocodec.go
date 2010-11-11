package socketio

import (
	"bytes"
	"container/vector"
	"fmt"
	"io"
	"json"
	"os"
	"strconv"
	"strings"
	"utf8"
)

// The various delimiters used for framing in the socket.io protocol.
const (
	sioFrameDelim          = "~m~"
	sioFrameDelimJSON      = "~j~"
	sioFrameDelimHeartbeat = "~h~"
	sioFrameDelimText      = ""
)

// SioMessage fulfills the message interface.
type sioMessage string

// MessageType checks if the message starts with sioFrameDelimJSON or
// sioFrameDelimHeartbeat. If the prefix is something else, then the message
// is interpreted as a basic messageText.
func (sm sioMessage) Type() uint8 {
	if len(sm) >= 3 {
		switch sm[0:3] {
		case sioFrameDelimJSON:
			return MessageJSON

		case sioFrameDelimHeartbeat:
			return MessageHeartbeat
		}
	}

	return MessageText
}

// Heartbeat looks for a heartbeat value in the message. If a such value
// can be extracted, then that value and a true is returned. Otherwise a
// false will be returned.
func (sm sioMessage) heartbeat() (heartbeat, bool) {
	var hb heartbeat
	if n, _ := fmt.Sscanf(string(sm), sioFrameDelimHeartbeat+"%d", &hb); n != 1 {
		return -1, false
	}

	return hb, true
}

// Data returns the raw message.
func (sm sioMessage) Data() string {
	return string(sm)
}

// JSON returns the JSON embedded in the message, if available.
func (sm sioMessage) JSON() (string, bool) {
	if sm.Type() == MessageJSON {
		return string(sm[3:]), true
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
		_, err = fmt.Fprintf(dst, "%s%d%s%s%d", sioFrameDelim, len(s)+len(sioFrameDelimHeartbeat), sioFrameDelim, sioFrameDelimHeartbeat, t)

	case handshake:
		_, err = fmt.Fprintf(dst, "%s%d%s%s", sioFrameDelim, len(t), sioFrameDelim, t)

	case []byte:
		l := utf8.RuneCount(t)
		if l == 0 {
			break
		}
		_, err = fmt.Fprintf(dst, "%s%d%s%s%s", sioFrameDelim, l+len(sioFrameDelimText), sioFrameDelim, sioFrameDelimText, t)

	case string:
		l := utf8.RuneCountInString(t)
		if l == 0 {
			break
		}
		_, err = fmt.Fprintf(dst, "%s%d%s%s%s", sioFrameDelim, l+len(sioFrameDelimText), sioFrameDelim, sioFrameDelimText, t)

	case int:
		s := strconv.Itoa(t)
		if s == "" {
			break
		}

		_, err = fmt.Fprintf(dst, "%s%d%s%s%s", sioFrameDelim, len(s)+len(sioFrameDelimText), sioFrameDelim, sioFrameDelimText, s)

	default:
		data, err := json.Marshal(payload)
		if len(data) == 0 || err != nil {
			break
		}
		err = json.Compact(elem, data)
		if err != nil {
			break
		}

		_, err = fmt.Fprintf(dst, "%s%d%s%s", sioFrameDelim, utf8.RuneCount(elem.Bytes())+len(sioFrameDelimJSON), sioFrameDelim, sioFrameDelimJSON)
		if err == nil {
			_, err = elem.WriteTo(dst)
		}
	}

	return err
}

// Decode takes a payload and tries to decode and split it into messages.
// If an error occurs during any stage of the decoding, an error will be returned.
// Each "frame" must have a header like this:
// <sioFrameDelim>[length in utf8 codepoints]<sioFrameDelim>.
func (c SIOCodec) Decode(payload []byte) (messages []Message, err os.Error) {
	var frameLen, codePoint, headerLen, n int
	// TODO: replace frames with a typed slice, so we can avoid the silly loop
	// at the end.
	var frames vector.StringVector
	var frame string
	delimLen := len(sioFrameDelim)
	str := string(payload)
	utf8str := utf8.NewString(str)
	codePoints := utf8str.RuneCount()

	for i, l := 0, len(str); i < l; {
		// frameLen is in utf8 codepoints
		if n, _ = fmt.Sscanf(str[i:], sioFrameDelim+"%d"+sioFrameDelim, &frameLen); n != 1 || frameLen < 0 {
			return nil, ErrMalformedPayload
		}

		headerLen = delimLen + strings.Index(str[i+delimLen:], sioFrameDelim) + delimLen
		codePoint += headerLen

		if codePoint+frameLen > codePoints {
			return nil, ErrMalformedPayload
		}

		frame = utf8str.Slice(codePoint, codePoint+frameLen)
		codePoint += frameLen
		i += headerLen + len(frame)
		frames.Push(frame)
	}

	messages = make([]Message, frames.Len())
	for i, f := range frames {
		messages[i] = sioMessage(f)
	}

	return messages, nil
}
