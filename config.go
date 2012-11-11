package socketio

import "time"

type Config struct {
	CloseTimeout      time.Duration
	HeartbeatInterval time.Duration
	HeartbeatTimeout  time.Duration
	PollingTimeout    time.Duration
	Transports        []*Transport
	WriteTimeout      time.Duration
}

var DefaultConfig = Config{
	CloseTimeout:      time.Duration(25e9),
	HeartbeatInterval: time.Duration(15e9),
	HeartbeatTimeout:  time.Duration(10e9),
	PollingTimeout:    time.Duration(20e9),
	Transports:        DefaultTransports,
	WriteTimeout:      time.Duration(5e9),
}
