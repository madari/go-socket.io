package socketio

import (
	"bytes"
	"errors"
	"io"
)

var (
	ErrMalformedPayload = errors.New("malformed payload")
)

// A Codec wraps Encode and Decode methods.
//
// Encode takes an interface{}, encodes it and writes it to the given io.Writer.
// Decode takes a slice of bytes and decodes them into messages. If the given payload
// can't be decoded, an ErrMalformedPayload error will be returned.
type Codec interface {
	NewEncoder() Encoder
	NewDecoder(*bytes.Buffer) Decoder
}

type Decoder interface {
	Decode() ([]Message, error)
	Reset()
}

type Encoder interface {
	Encode(io.Writer, interface{}) error
}
