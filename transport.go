package socketio

import (
	"errors"
	"fmt"
	"io"
	"net/http"
)

var (
	// ErrNotConnected is used when some action required the connection to be online,
	// but it wasn't.
	ErrNotConnected = errors.New("not connected")

	// ErrConnected is used when some action required the connection to be offline,
	// but it wasn't.
	ErrConnected = errors.New("already connected")

	emptyResponse = []byte{}
	okResponse    = []byte("ok")
)

// DefaultTransports holds the defaults
var DefaultTransports = []Transport{
	NewXHRPollingTransport(10e9, 5e9),
	NewXHRMultipartTransport(0, 5e9),
	NewWebsocketTransport(0, 5e9),
	NewHTMLFileTransport(0, 5e9),
	NewFlashsocketTransport(0, 5e9),
	NewJSONPPollingTransport(0, 5e9),
}

// Transport is the interface that wraps the Resource and newSocket methods.
//
// Resource returns the resource name of the transport, e.g. "websocket".
// NewSocket creates a new socket that embeds the corresponding transport
// mechanisms.
type Transport interface {
	Resource() string
	newSocket() socket
}

// Socket is the interface that wraps the basic Read, Write, Close and String
// methods. Additionally it has Transport and accept methods.
// 
// Transport returns the Transport that created this socket.
// Accept takes the http.ResponseWriter / http.Request -pair from a http handler
// and hijacks the connection for itself. The third parameter is a function callback
// that will be invoked when the connection has been succesfully hijacked and the socket
// is ready to be used.
type socket interface {
	io.ReadWriteCloser
	fmt.Stringer

	Transport() Transport
	accept(http.ResponseWriter, *http.Request, func()) error
}
