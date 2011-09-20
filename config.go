package socketio

type Config struct {
	CloseTimeout         int64
	HeartbeatInterval    int64
	HeartbeatTimeout     int64
	PollingTimeout       int64
	Transports           []*Transport
	WriteTimeout         int64
}

var DefaultConfig = Config{
	CloseTimeout:         25e9,
	HeartbeatInterval:    15e9,
	HeartbeatTimeout:     10e9,
	PollingTimeout:       20e9,
	Transports:           DefaultTransports,
	WriteTimeout:         5e9,
}
