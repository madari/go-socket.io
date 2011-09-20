package socketio

import (
	"bytes"
	"http"
	"io/ioutil"
	"net"
	"os"
	"sync"
	"time"
)

var (
	ErrDisconnected = os.NewError("connection is disconnected")
)

type Conn struct {
	connect          chan byte
	dec              *Decoder
	disconnect       chan byte
	disconnected     bool
	flush            chan chan os.Error
	incoming         chan []byte
	mutex            sync.Mutex
	online           bool
	pendingHeartbeat bool
	queue            []interface{}
	server           *server
	shutdown         chan byte
	sid              string
	socket           Socket
	transport        *Transport
}

func newConn(server *server) (c *Conn, err os.Error) {
	var sid string
	if sid, err = newSessionId(); err != nil {
		return nil, err
	}

	c = &Conn{
		connect:    make(chan byte),
		dec:        &Decoder{},
		disconnect: make(chan byte),
		flush:      make(chan chan os.Error, 1),
		incoming:   make(chan []byte),
		server:     server,
		shutdown:   make(chan byte),
		sid:        sid,
	}

	go c.machine()
	return
}

func (c *Conn) Emit(name string, args ...interface{}) os.Error {
	return c.Send(&event{Name: name, Args: args})
}

func (c *Conn) Receive(msg *Message) (err os.Error) {
	for {
		if err = c.dec.Decode(msg); err == os.EOF {
			if payload, ok := <-c.incoming; ok {
				c.dec.Write(payload)
				continue
			} else {
				return os.EOF
			}
		} else if err != nil {
			break
		}

		switch msg.typ {
		case MessageHeartbeat:
			Log.debug(c, " receive: received heartbeat: ", msg.Inspect())
			c.mutex.Lock()
			c.pendingHeartbeat = false
			c.mutex.Unlock()

		case MessageDisconnect:
			Log.info(c, " receive: received disconnect: ", msg.Inspect())
			c.mutex.Lock()
			if !c.disconnected {
				if c.socket != nil {
					c.socket.Close()
				}
				c.shutdown <- 1
			}
			c.mutex.Unlock()
			return os.EOF

		case MessageConnect, MessageError, MessageACK, MessageNOOP:
			Log.warn(c, " receive: (TODO) ", msg.Inspect())

		case MessageEvent, MessageText, MessageJSON:
			if msg.id > 0 && !msg.ack {
				Log.debug(c, " receive: automatically acking: ", msg.Inspect())
				c.mutex.Lock()
				if c.disconnected {
					Log.warn(c, " receive: unable to ack since disconnected: ", msg.Inspect())
					c.mutex.Unlock()
					return os.EOF
				}
				c.dispatch(&ack{id: msg.id})
				c.mutex.Unlock()
			}
			return

		default:
			Log.warn(c, " receive: unknown message type: ", msg.Inspect())
		}
	}

	c.dec.Reset()
	return
}

func (c *Conn) Reply(m *Message, a ...interface{}) os.Error {
	ack := &ack{
		id:   m.id,
		data: a,
	}
	if len(a) > 0 {
		ack.event = true
	}
	return c.Send(ack)
}

func (c *Conn) Send(data interface{}) os.Error {
	c.mutex.Lock()
	if c.disconnected {
		c.mutex.Unlock()
		return ErrDisconnected
	}
	c.dispatch(data)
	c.mutex.Unlock()
	return nil
}

func (c *Conn) SendWait(data interface{}) <-chan os.Error {
	res := make(chan os.Error, 1)
	c.mutex.Lock()
	if c.disconnected {
		c.mutex.Unlock()
		res <- ErrDisconnected
		close(res)
		return res
	}
	c.queue = append(c.queue, data)
	c.flush <- res
	c.mutex.Unlock()
	return res
}

func (c *Conn) dispatch(data interface{}) {
	c.queue = append(c.queue, data)
	select {
	case c.flush <- nil:
	default:
	}
}

func (c *Conn) String() string {
	return c.sid
}

func (c *Conn) close() {
	if c.Send(disconnect("")) == ErrDisconnected {
		return
	}
	c.shutdown <- 1
}

func (c *Conn) handle(t *Transport, w http.ResponseWriter, req *http.Request) (err os.Error) {
	c.mutex.Lock()

	if c.disconnected {
		c.mutex.Unlock()
		return ErrDisconnected
	}

	if req.Method == "POST" {
		c.mutex.Unlock()

		var payload []byte
		if t.PostEncoded {
			payload = []byte(req.FormValue("d"))
		} else {
			payload, err = ioutil.ReadAll(req.Body)
			req.Body.Close()
		}

		if err == nil {
			Log.debugf("%s handle: received: %s", c, payload)
			c.incoming <- payload
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("1"))
		} else {
			Log.warn(c, " handle: unable to read post body: ", err)
		}

		return
	}

	err = t.Hijack(w, req, func(socket Socket) {
		if t.Type == PollingTransport {
			socket.SetReadTimeout(c.server.config.PollingTimeout)
		}
		socket.SetWriteTimeout(c.server.config.WriteTimeout)
		if c.socket != nil {
			c.socket.Close()
		}
		c.socket = socket
		c.transport = t
		c.connect <- 1
		c.mutex.Unlock()
		c.drain(socket)
		c.disconnect <- 1
	})

	if err != nil {
		c.mutex.Unlock()
	}

	return
}

func (c *Conn) flusher() {
	var buf bytes.Buffer
	var err os.Error
	enc := &Encoder{}

	for res := range c.flush {
		c.mutex.Lock()
		if err := enc.Encode(&buf, c.queue); err != nil {
			Log.warn(c, " flusher: encode error: ", err)
		}
		c.queue = c.queue[:0]
		socket := c.socket
		c.mutex.Unlock()

		for {
			if _, err = buf.WriteTo(socket); err == nil {
				break
			} else if err != os.EAGAIN {
				Log.info(c, " flusher: write error: ", err)
				break
			}
		}
		if res != nil {
			res <- err
			close(res)
		}
	}
}

func (c *Conn) drain(socket Socket) {
	var buf []byte
	defer socket.Close()

	for {
		err := socket.Receive(&buf)
		if err != nil {
			if err != os.EAGAIN {
				if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
					<-c.SendWait(noop(0))
					Log.debug(c, " drain: lost connection (read timeout, noop sent)")
				} else {
					Log.debug(c, " drain: lost connection: ", err)
				}
				break
			}
		} else if len(buf) > 0 {
			c.mutex.Lock()
			if c.disconnected {
				c.mutex.Unlock()
				return
			}
			c.incoming <- buf
			c.mutex.Unlock()
		}
	}
}

func (c *Conn) machine() {
	numConns := 0

	defer func() {
		Log.debug(c, " machine: shutting down")
		c.mutex.Lock()
		c.disconnected = true
		c.online = false
		c.pendingHeartbeat = false
		c.socket = nil
		close(c.connect)
		close(c.disconnect)
		close(c.flush)
		close(c.incoming)
		close(c.shutdown)
		c.mutex.Unlock()
	}()

	var ticker *time.Ticker
	wait := func(ns int64) <-chan int64 {
		if ticker != nil {
			ticker.Stop()
		}
		ticker = time.NewTicker(ns)
		return ticker.C
	}

	for {
		Log.debug(c, " machine: waiting for connection")

		select {
		case <-c.shutdown:
			Log.debug(c, " machine: got shutdown")
			return

		case <-c.connect:
			Log.debugf("%s machine: online using %s (addr=%s, reconnect=%d)", c, c.transport.Name, c.socket.RemoteAddr(), numConns)
			c.online = true

			numConns++
			if numConns == 1 {
				go c.flusher()
				c.Send(connect(""))
			} else {
				select {
				case c.flush <- nil:
				default:
				}
			}

		Connected:
			for {
				if c.server.config.HeartbeatInterval <= 0 || c.transport.Type != StreamingTransport {
					<-c.disconnect
					break Connected
				} else {
					select {
					case <-c.disconnect:
						break Connected

					case <-wait(c.server.config.HeartbeatInterval):
						Log.debug(c, " machine: hit heartbeat interval, sending heartbeat and scheduling timeout")
						c.mutex.Lock()
						c.pendingHeartbeat = true
						c.dispatch(heartbeat(0))
						c.mutex.Unlock()
						time.AfterFunc(c.server.config.HeartbeatTimeout, func() {
							c.mutex.Lock()
							defer c.mutex.Unlock()
							if c.pendingHeartbeat {
								Log.debug(c, " machine: heartbeat timeout fired")
								c.socket.Close()
								c.shutdown <- 1
								return
							}
						})
					}
				}
			}

			c.online = false
			Log.debug(c, " machine: offline")

		case <-wait(c.server.config.CloseTimeout):
			Log.debug(c, " machine: close timeout fired")
			return
		}
	}
}
