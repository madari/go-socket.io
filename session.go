package socketio

import (
	"crypto/rand"
	"io"
)

// SessionID is just a string for now.
type SessionID string

const (
	// Length of the session ids.
	SessionIDLength = 16

	// Charset from which to build the session ids.
	SessionIDCharset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

// NewSessionID creates a new ~random session id that is SessionIDLength long and
// consists of random characters from the SessionIDCharset.
func NewSessionID() (sid SessionID, err error) {
	b := make([]byte, SessionIDLength)

	if _, err = io.ReadFull(rand.Reader, b); err != nil {
		return
	}

	for i := 0; i < SessionIDLength; i++ {
		b[i] = SessionIDCharset[b[i]%uint8(len(SessionIDCharset))]
	}

	sid = SessionID(b)
	return
}
