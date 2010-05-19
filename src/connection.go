package socketio

import (
	"http"
	"os"
	"container/vector"
	"crypto/rand"
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
	queue      *vector.Vector // holds the undelivered messages
	tLock      *dMutex        // protects the i/o connection
	queueLock  *dMutex        // protects the message queue
	numConns   int            // total number of requests
	handshaked bool           // is the handshake sent
	destroyed  bool           // is the conn destroyed
	wakeup     chan byte      // used internally to wake up the flusher
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
		wakeup:    make(chan byte),
		queue:     new(vector.Vector),
		tLock:     newDMutex("transport:" + sessionid),
		queueLock: newDMutex("queue:" + sessionid),
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
		c.queueLock.Lock("c.Send")
		if c.queue.Len() < ConnMaxBuffer {
			c.queue.Push(data)
		} else {
			err = ErrBufferFull
		}
		c.queueLock.Unlock()

		_ = c.wakeup <- 1
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

		c.tLock.Lock("c.reconnect")
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
	c.queueLock.Lock("destroy")
	c.tLock.Lock("destroy")
	c.destroyed = true
	c.sio.onDisconnect(c)
	c.tLock.Unlock()
	c.queueLock.Unlock()

	close(c.wakeup)
}

// Flusher waits for signals on the wakeup-channel and tries to
// flush the message queue to the underlaying transport
func (c *Conn) flusher() {
	// wait for a signal
	for _ = range c.wakeup {
		if closed(c.wakeup) {
			return
		}

		c.queueLock.Lock("flusher")
		if l := c.queue.Len(); l > 0 {

			// queue had messages in it so let's encode them
			if data, err := c.sio.formatter.PayloadEncoder(c.queue); err != nil {
				// encoding failed
				// TODO: gracefull handling
				c.sio.Log("conn/flusher: payloadEncoder:", err.String())
				c.queue = c.queue.Resize(0, 0)
			} else {
				// encoded payload is now in p, so let's try to write it
				c.tLock.Lock("flusher")
				if _, err := c.t.Write(data); err != nil {
					// write failed, so let's not do anything. A new delivery
					// will be attempted after the next signal
					c.sio.Log("conn/flusher: write:", err.String())
				} else {
					// write succeeded, so let's clear the message queue and notify
					// the listeners
					c.queue = c.queue.Resize(0, 0)
				}
				c.tLock.Unlock()
			}
		}
		c.queueLock.Unlock()
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
		c.tLock.Lock("c.onConnect")
		_, err = c.t.Write(data)
		if err != nil {
			c.sio.Log("conn/onConnect: write:", err.String())
			c.t.Close()
			return
		}
		c.tLock.Unlock()

		// notify the sio about a new connection
		c.sio.onConnect(c)
	}

	c.numConns++

	_ = c.wakeup <- 1
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
