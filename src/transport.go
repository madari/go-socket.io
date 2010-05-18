package socketio

import (
	"http"
	"os"
	"io"
)

// DefaultTransportConfigSet holds the defaults
var DefaultTransportConfigSet = []TransportConfig{
	XHRPollingTransportConfig(120e9),
	XHRMultipartTransportConfig(0),
	WebsocketTransportConfig(0, true),
}

// TransportConfig is a transport factory for a certain transport type.
type TransportConfig interface {
	Resource() string             // returns the resource name, e.g. "websocket"
	newTransport(*Conn) transport // creates a new transport to be used with c
}

// Transport is the internal interface describing transport instances
type transport interface {
	io.Writer // implements the io.Writer interface
	io.Closer // implements the io.Closer interface

	String() string                            // verbose description of the transport instance
	Config() TransportConfig                   // return the configuration in use
	handle(*http.Conn, *http.Request) os.Error // used internally to assign http-connections
}
