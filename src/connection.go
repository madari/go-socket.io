package socketio

import (
	"http"
	"os"
	"crypto/rand"
	"sync"
	"io"
)

const (
	ConnMaxWaitTicks = 5  // allow clients to reconnect in max 5 ticks
	ConnMaxBuffer    = 20 // amount of messages to persist at most
	SessionIDLength  = 16 // length of the session id
	SessionIDCharset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

var (
	ErrDestroyed  = os.NewError("The connection was destroyed")
	ErrBufferFull = os.NewError("The connection buffer was full")
)

// NewSessionID creates a new session id consisting of
// of SessionIDLength chars from the SessionIDCharset
func newSessionID() (sessionid string, err os.Error) {
	b := make([]byte, SessionIDLength)

	if _, err = io.ReadFull(rand.Reader, b); err != nil {
		return
	}

	clen := uint8(len(SessionIDCharset))

	for i := 0; i < SessionIDLength; i++ {
		b[i] = SessionIDCharset[b[i]%clen]
	}

	sessionid = string(b)
	return
}

// Conn represents a single session and handles its handshaking,
// message buffering and reconnections
type Conn struct {
	t          transport // the i/o connection
	sio        *SocketIO // the server
	sessionid  string
	queue      chan interface{} // buffers the outgoing messages
	tLock      *sync.Mutex    // protects the i/o connection
	numConns   int            // total number of requests
	handshaked bool           // is the handshake sent
	destroyed  bool           // is the conn destroyed
	heartbeat  chan byte      // used internally to wake up the flusher
}

// Returns a new connection to be used with sio
func newConn(sio *SocketIO) (c *Conn, err os.Error) {
	var sessionid string
	if sessionid, err = newSessionID(); err != nil {
		sio.Log("newConn: newSessionID:", err.String())
		return
	}

	c = &Conn{
		sio:       sio,
		sessionid: sessionid,
		heartbeat: make(chan byte),
		queue:     make(chan interface{}, ConnMaxBuffer),
		tLock:     new(sync.Mutex),
	}

	return
}


//// PUBLIC INTERFACE ////

// Returns a string representation of the connection
func (c *Conn) String() string {
	return "{" + c.sessionid + "," + c.t.String() + "}"
}

// Send queues data for a delivery. It is totally content agnostic with one exception:
// the data given must be marshallable by the standard-json package. If the buffer
// has reached ConnMaxBuffer, then the data is dropped and a false is returned.
func (c *Conn) Send(data interface{}) (err os.Error) {
	if !c.destroyed {
		if ok := c.queue <- data; !ok {
			err = ErrBufferFull
		}
	} else {
		err = ErrDestroyed
	}

	return
}


//// INTERNAL IMPLEMENTATION  ////

// Handle takes over an http conn/req -pair using a TransportConfig.
// If the connection already has the same kind of transport assigned to it,
// it re-uses it, but if the transports don't match then a new transport is created
// using the tc.
func (c *Conn) handle(tc TransportConfig, conn *http.Conn, req *http.Request) (err os.Error) {
	if c.destroyed {
		return os.EOF
	}

	if c.t != nil && tc == c.t.Config() {
		err = c.t.handle(conn, req)
	} else {
		go c.flusher()

		c.tLock.Lock()
		if c.t != nil {
			c.t.Close()
		}
		transport := tc.newTransport(c)
		c.t = transport
		c.tLock.Unlock()

		err = transport.handle(conn, req)
	}
	return
}


// The destroyer handles the final destroying of the actual
// connection, after its network connection was lost. It first waits for
// ConnMaxWaitTicks and checks if the client has reconnected. If so
// then it aborts the destroying.
func (c *Conn) destroyer() {
	numConns := c.numConns

	slot := <-c.sio.ticker.Register
	defer func() {
		c.sio.ticker.Unregister <- slot
	}()

	for i := 0; i < ConnMaxWaitTicks; i++ {
		<-slot.Value.(chan interface{})

		if c.numConns > numConns {
			return
		}
	}

	c.destroy()
}

// Destroy marks the connection destroyed and notifies the sio
// about the disconnection
func (c *Conn) destroy() {
	c.tLock.Lock()
	c.destroyed = true
	c.sio.onDisconnect(c)
	c.tLock.Unlock()

	close(c.heartbeat)
}

// Flusher waits for messages on the queue. It then
// tries to write the messages to the underlaying transport and
// will keep on trying until the heartbeat is killed or the payload
// can be delivered. It is responsible for persisting messages until they
// can be succesfully delivered. No more than ConnMaxBuffer messages should
// ever be waiting for delivery.
// NOTE: the ConnMaxBuffer is not a "hard limit", because one could have
// max amount of messages waiting in the queue and in the payload itself
// simultaneously.
func (c *Conn) flusher() {
	payload := make([]interface{}, ConnMaxBuffer)

	for msg := range c.queue {
		payload[0] = msg

		n := 1
		for {
			msg, ok := <-c.queue
			if !ok || n >= ConnMaxBuffer {
				break
			}

			payload[n] = msg
			n++
		}

		data, err := c.sio.formatter.PayloadEncoder(payload[0:n])
		if err != nil {
			c.sio.Log("conn/flusher: payloadEncoder:", err.String())
			continue
		}

		L: for {
			for {
				c.tLock.Lock()
				_, err = c.t.Write(data)
				c.tLock.Unlock()

				if err == nil {
					break L
				} else if err != os.EAGAIN {
					c.sio.Log("conn/flusher: write:", err.String())
					break
				}
			}

			<-c.heartbeat
			if closed(c.heartbeat) {
				return
			}
		}
	}
}


//// TRANSPORT EVENTS ////

// OnConnect is called by transports after the i/o connection has been established.
// The first thing to do is to try to send the handshake and notify the owner sio about
// a new connection. If the handshake was already sent, then this is just a reconnection.
func (c *Conn) onConnect() {
	if c.destroyed {
		return
	}

	if !c.handshaked {
		c.handshaked = true

		data, err := c.sio.formatter.HandshakeEncoder(c)
		if err != nil {
			c.sio.Log("conn/onConnect: handshakeEncoder:", err.String())
			c.t.Close()
			return
		}

		// bypass the message queue and send the handshake
		c.tLock.Lock()
		_, err = c.t.Write(data)
		if err != nil {
			c.sio.Log("conn/onConnect: write:", err.String())
			c.t.Close()
			c.tLock.Unlock()
			return
		}
		c.tLock.Unlock()

		// notify the sio about a new connection
		c.sio.onConnect(c)
	}

	c.numConns++
	_ = c.heartbeat <- 1
}

// OnDisconnect is invoked by the transport. It creates a destroyer, which
// waits for reconnection
func (c *Conn) onDisconnect() {
	if !c.destroyed {
		go c.destroyer()
	}
}

// OnMessage is invoked by the transport. It takes a raw message payload,
// unmarshalls it and invokes the sio's onMessage handle per each individual message.
func (c *Conn) onMessage(data []byte) {
	if !c.destroyed {
		msgs, err := c.sio.formatter.PayloadDecoder(data)
		if err != nil {
			c.sio.Log("conn/onMessage: payloadDecoder:", err.String())
			return
		}

		for _, m := range msgs {
			c.sio.onMessage(c, m)
		}
	}
}
