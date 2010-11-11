package socketio

import (
	"io"
	"os"
)

var (
	ErrMalformedPayload = os.NewError("malformed payload")
)

// A Codec wraps Encode and Decode methods.
//
// Encode takes an interface{}, encodes it and writes it to the given io.Writer.
// Decode takes a slice of bytes and decodes them into messages. If the given payload
// can't be decoded, an ErrMalformedPayload error will be returned.
type Codec interface {
	Encode(io.Writer, interface{}) os.Error
	Decode([]byte) ([]Message, os.Error)
}
