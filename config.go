package socketio

import "log"

// Config represents a set of configurable settings used by the server
type Config struct {
	// Maximum number of connections.
	MaxConnections int

	// Maximum amount of messages to store for a connection. If a connection
	// has QueueLength amount of undelivered messages, the following Sends will
	// return ErrQueueFull error.
	QueueLength int

	// The size of the read buffer in bytes.
	ReadBufferSize int

	// The interval between heartbeats
	HeartbeatInterval int64

	// Period in ns during which the client must reconnect or it is considered
	// disconnected.
	ReconnectTimeout int64

	// Origins to allow for cross-domain requests.
	// For example: ["localhost:8080", "myblog.com:*"].
	Origins []string

	// Transports to use.
	Transports []Transport

	// Codec to use.
	Codec Codec

	// The resource to bind to, e.g. /socket.io/
	Resource string

	// Logger to use.
	Logger *log.Logger
}

var DefaultConfig = Config{
	MaxConnections:    0,
	QueueLength:       10,
	ReadBufferSize:    2048,
	HeartbeatInterval: 10e9,
	ReconnectTimeout:  10e9,
	Origins:           nil,
	Transports:        DefaultTransports,
	Codec:             SIOCodec{},
	Resource:          "/socket.io/",
	Logger:            DefaultLogger,
}
