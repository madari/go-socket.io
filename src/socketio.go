/*
	The socketio package is a simple abstraction layer for different web browser-
	supported transport mechanisms. It is meant to be fully compatible with the
	Socket.IO client side JavaScript socket API library by LearnBoost Labs
	(http://socket.io/), but through custom formatters it should suit for any client
	implementation.

	It (together with the LearnBoost's client-side libraries) provides an easy way for
	developers to access the most popular browser transport mechanism today:
	multipart- and long-polling XMLHttpRequests, HTML5 WebSockets and
	forever-frames [TODO]. The socketio package works hand-in-hand with the standard
	http package (by plugging itself into a configurable ServeMux) and hence it
	doesn't need a full network port for itself. It has an callback-style event
	handling API. The callbacks are:

		- SocketIO.OnConnect
		- SocketIO.OnDisconnect
		- SocketIO.OnMessage

	Other utility-methods include:

		- SocketIO.Mux
		- SocketIO.Broadcast
		- SocketIO.BroadcastExcept
		- SocketIO.IterConns
		- SocketIO.GetConn

	Each new connection will be automatically assigned an unique session id and
	using those the clients can reconnect without losing messages: the server
	persists clients' pending messages (until some configurable point) if they can't
	be immediately delivered. All writes are by design asynchronous and can be made
	through Conn.Send.

	Finally, the actual format on the wire is described by a separate `Formatter`.
	The default formatter is compatible with the LearnBoost's
	Socket.IO client.

	For example, here is a simple chat server:

		package main

		import (
			"http"
			"log"
			"socketio"
		)

		func main() {
			sio := socketio.NewSocketIO(nil)
			sio.Mux("/socket.io/", nil)

			http.Handle("/", http.FileServer("www/", "/"))

			sio.OnConnect(func(c *socketio.Conn) {
				sio.Broadcast(struct{ announcement string }{"connected: " + c.String()})
			})

			sio.OnDisconnect(func(c *socketio.Conn) {
				sio.BroadcastExcept(c, struct{ announcement string }{"disconnected: " + c.String()})
			})

			sio.OnMessage(func(c *socketio.Conn, msg string) {
				sio.BroadcastExcept(c,
					struct{ message []string }{[]string{c.String(), msg}})
			})

			log.Stdout("Server started.")
			if err := http.ListenAndServe(":8080", nil); err != nil {
				log.Stdout("ListenAndServe: %s", err.String())
				os.Exit(1)
			}
		}
*/
package socketio

import (
	"http"
	"os"
	"log"
	"strings"
	"time"
)

// SocketIO handles transport abstraction and provide the user
// a handfull of callbacks to observe different events.
type SocketIO struct {
	*log.Logger
	sessions     map[string]*Conn  // holds the outstanding sessions
	sessionsLock *dMutex           // protects the sessions
	transports   []TransportConfig // holds the configurable transport set
	ticker       *Broadcaster      // a ticker providing signals every second
	formatter    Formatter         // encode & decode on-the-wire format

	// Holds the callbacks set by the user
	callbacks struct {
		onConnect    func(*Conn)         // invoked on new connection
		onDisconnect func(*Conn)         // invoked on a lost connection
		onMessage    func(*Conn, string) // invoked on a message
	}
}


//// PUBLIC INTERFACE ////

// NewSocketIO creates a new socketio server with configurable transports.
// If transports is nil, the DefaultTransportConfigSet is used.
func NewSocketIO(transports []TransportConfig) (sio *SocketIO) {
	if transports == nil {
		transports = DefaultTransportConfigSet
	}

	sio = &SocketIO{
		Logger:       DefaultLogger,
		transports:   transports,
		sessions:     make(map[string]*Conn),
		sessionsLock: newDMutex("sessions"),
		ticker:       NewBroadcaster(),
		formatter:    DefaultFormatter{},
	}

	// start broadcasting ticks every second
	go func() {
		ticker := time.NewTicker(1e9)
		for _ = range ticker.C {
			sio.ticker.Write <- byte(1)
		}
	}()

	return
}

// Broadcast schedules data to be sent to each connection.
func (sio *SocketIO) Broadcast(data interface{}) {
	sio.BroadcastExcept(nil, data)
}

// BroadcastExcept schedules data to be sent to each connection except
// c. It does not care about the type of data, but it must marshallable
// by the standard json-package.
func (sio *SocketIO) BroadcastExcept(c *Conn, data interface{}) {
	for v := range sio.IterConns() {
		if v != c {
			v.Send(data)
		}
	}
}

// IterConns creates a thread-safe channel (by blocking sessions!) for
// iterating the connections.
func (sio *SocketIO) IterConns() <-chan *Conn {
	c := make(chan *Conn)

	go func() {
		sio.sessionsLock.Lock("sio.IterConns")
		for _, conn := range sio.sessions {
			if closed(c) {
				break
			}
			c <- conn
		}
		sio.sessionsLock.Unlock()
		close(c)
	}()

	return c
}

// GetConn is a tread-safe way for looking up a connection by its session id
func (sio *SocketIO) GetConn(sessionid string) (c *Conn) {
	sio.sessionsLock.Lock("sio.GetConn")
	c = sio.sessions[sessionid]
	sio.sessionsLock.Unlock()
	return
}

// Mux maps resources to the http.ServeMux mux under the resource given.
// The resource must end with a slash and if the mux is nil, the
// http.DefaultServeMux is used. It registers handlers for URLs like:
// <resource><t.resource>[/], e.g. /socket.io/websocket && socket.io/websocket/.
func (sio *SocketIO) Mux(resource string, mux *http.ServeMux) {
	if mux == nil {
		mux = http.DefaultServeMux
	}

	if resource == "" || resource[len(resource)-1] != '/' {
		panic("resource must end with a slash")
	}

	for _, t := range sio.transports {
		s := t
		tresource := resource + s.Resource()
		mux.HandleFunc(tresource+"/", func(c *http.Conn, req *http.Request) {
			sio.handle(s, c, req)
		})
		mux.HandleFunc(tresource, func(c *http.Conn, req *http.Request) {
			sio.handle(s, c, req)
		})
	}
}

// Sets the logger
func (sio *SocketIO) SetLogger(l *log.Logger) {
	if l == nil {
		panic("can't be nil")
	}
	sio.Logger = l
}

// Sets the formatter
func (sio *SocketIO) SetFormatter(f Formatter) {
	sio.formatter = f
}


//// PUBLIC CALLBACKS ////

// OnConnect sets f to be invoked when a new session is established. It passes
// the established connection as an argument to the callback.
// NOTE: the callback should not block
func (sio *SocketIO) OnConnect(f func(*Conn)) {
	sio.callbacks.onConnect = f
}

// OnDisconnect sets f to be invoked when a session is considered to be lost. It passes
// the established connection as an argument to the callback. After disconnection
// the connection is considered to be destroyed, and it should not be used anymore.
// NOTE: the callback should not block
func (sio *SocketIO) OnDisconnect(f func(*Conn)) {
	sio.callbacks.onDisconnect = f
}

// OnMessage sets f to be invoked when a message arrives. It passes
// the established connection along with the received message as arguments
// to the callback.
// NOTE: the callback should not block
func (sio *SocketIO) OnMessage(f func(*Conn, string)) {
	sio.callbacks.onMessage = f
}


//// PRIVATE IMPLEMENTATION ////

// Handle is invoked on every http-request coming through the muxer.
// It is responsible for parsing the request and passing http conn/req -pair
// to the corresponding connections. It also creates new connections if
// the URL does not contain a valid session id. The URL and method must be
// one of the following:
//
//  GET resource
//  GET resource/sessionid
// POST resource/sessionid/send
//
func (sio *SocketIO) handle(t TransportConfig, conn *http.Conn, req *http.Request) {
	var parts []string
	var connection *Conn
	var sessionid string
	var err os.Error

	// TODO: fails if the session id matches the transport
	if i := strings.LastIndex(req.URL.Path, t.Resource()); i >= 0 {
		parts = strings.Split(req.URL.Path[i:], "/", 0)
	}

	switch len(parts) {
	case 1:
		// only resource was present, so create a new connection
		connection, err = newConn(sio)
		if err != nil {
			sio.Log("sio/handle: newConn:", err.String())
			conn.WriteHeader(http.StatusInternalServerError)
			return
		}
		sessionid = connection.sessionid
		sio.setConn(sessionid, connection)

	case 2:
		fallthrough

	case 3:
		// session id was present
		connection = sio.GetConn(parts[1])
	}

	// we should now have a connection
	if connection == nil {
		sio.Log("sio/handle: bad request:", req.RawURL)
		conn.WriteHeader(http.StatusBadRequest)
		return
	}

	// pass the http conn/req pair to the connection
	if err = connection.handle(t, conn, req); err != nil {
		sio.Log("sio/handle:", err.String())
		conn.WriteHeader(http.StatusInternalServerError)
	} else {
		sio.Log("sio/handle:", connection.String())
	}
}

// SetConn stores the connection c by sessionid in a thread-safe way
func (sio *SocketIO) setConn(sessionid string, c *Conn) {
	sio.sessionsLock.Lock("sio.setConn")
	sio.sessions[sessionid] = c
	sio.sessionsLock.Unlock()
}


//// CONNECTION CALLBACKS ////

// OnConnect is invoked by a connection when a new connection has been
// established succesfully. The establised connection is passed as an
// argument. It calls the user's OnConnect callback.
func (sio *SocketIO) onConnect(c *Conn) {
	sio.Log("sio/onConnect:", c.String())
	if sio.callbacks.onConnect != nil {
		sio.callbacks.onConnect(c)
	}
}

// OnDisconnect is invoked by a connection when the connection is considered
// to be lost. It calls the user's OnDisconnect callback.
func (sio *SocketIO) onDisconnect(c *Conn) {
	sio.Log("sio/onDisconnect:", c.String())
	sio.sessionsLock.Lock("sio.onDisconnect")
	sio.sessions[c.sessionid] = nil, false
	sio.sessionsLock.Unlock()

	if sio.callbacks.onConnect != nil {
		sio.callbacks.onDisconnect(c)
	}
}

// OnMessage is invoked by a connection when a new message arrives. It passes
// this message to the user's OnMessage callback.
func (sio *SocketIO) onMessage(c *Conn, msg string) {
	sio.Log("sio/onMessage:", c.String())
	if sio.callbacks.onConnect != nil {
		sio.callbacks.onMessage(c, msg)
	}
}
