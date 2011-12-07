package socketio

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
)

// SocketIO handles transport abstraction and provide the user
// a handfull of callbacks to observe different events.
type SocketIO struct {
	sessions        map[SessionID]*Conn // Holds the outstanding sessions.
	sessionsLock    *sync.RWMutex       // Protects the sessions.
	config          Config              // Holds the configuration values.
	serveMux        *ServeMux
	transportLookup map[string]Transport

	// The callbacks set by the user
	callbacks struct {
		onConnect    func(*Conn)              // Invoked on new connection.
		onDisconnect func(*Conn)              // Invoked on a lost connection.
		onMessage    func(*Conn, Message)     // Invoked on a message.
		isAuthorized func(*http.Request) bool // Auth test during new http request
	}
}

// NewSocketIO creates a new socketio server with chosen transports and configuration
// options. If transports is nil, the DefaultTransports is used. If config is nil, the
// DefaultConfig is used.
func NewSocketIO(config *Config) *SocketIO {
	if config == nil {
		config = &DefaultConfig
	}

	sio := &SocketIO{
		config:          *config,
		sessions:        make(map[SessionID]*Conn),
		sessionsLock:    new(sync.RWMutex),
		transportLookup: make(map[string]Transport),
	}

	for _, t := range sio.config.Transports {
		sio.transportLookup[t.Resource()] = t
	}

	sio.serveMux = NewServeMux(sio)

	return sio
}

// Broadcast schedules data to be sent to each connection.
func (sio *SocketIO) Broadcast(data interface{}) {
	sio.BroadcastExcept(nil, data)
}

// BroadcastExcept schedules data to be sent to each connection except
// c. It does not care about the type of data, but it must marshallable
// by the standard json-package.
func (sio *SocketIO) BroadcastExcept(c *Conn, data interface{}) {
	sio.sessionsLock.RLock()
	defer sio.sessionsLock.RUnlock()

	for _, v := range sio.sessions {
		if v != c {
			v.Send(data)
		}
	}
}

// GetConn digs for a session with sessionid and returns it.
func (sio *SocketIO) GetConn(sessionid SessionID) (c *Conn) {
	sio.sessionsLock.RLock()
	c = sio.sessions[sessionid]
	sio.sessionsLock.RUnlock()
	return
}

// Mux maps resources to the http.ServeMux mux under the resource given.
// The resource must end with a slash and if the mux is nil, the
// http.DefaultServeMux is used. It registers handlers for URLs like:
// <resource><t.resource>[/], e.g. /socket.io/websocket && socket.io/websocket/.
func (sio *SocketIO) ServeMux() *ServeMux {
	return sio.serveMux
}

// OnConnect sets f to be invoked when a new session is established. It passes
// the established connection as an argument to the callback.
func (sio *SocketIO) OnConnect(f func(*Conn)) error {
	sio.callbacks.onConnect = f
	return nil
}

// OnDisconnect sets f to be invoked when a session is considered to be lost. It passes
// the established connection as an argument to the callback. After disconnection
// the connection is considered to be destroyed, and it should not be used anymore.
func (sio *SocketIO) OnDisconnect(f func(*Conn)) error {
	sio.callbacks.onDisconnect = f
	return nil
}

// OnMessage sets f to be invoked when a message arrives. It passes
// the established connection along with the received message as arguments
// to the callback.
func (sio *SocketIO) OnMessage(f func(*Conn, Message)) error {
	sio.callbacks.onMessage = f
	return nil
}

// SetAuthorization sets f to be invoked when a new http request is made. It passes
// the http.Request as an argument to the callback.
// The callback should return true if the connection is authorized or false if it
// should be dropped. Not setting this callback results in a default pass-through.
func (sio *SocketIO) SetAuthorization(f func(*http.Request) bool) error {
	sio.callbacks.isAuthorized = f
	return nil
}

func (sio *SocketIO) Log(v ...interface{}) {
	if sio.config.Logger != nil {
		sio.config.Logger.Println(v...)
	}
}

func (sio *SocketIO) Logf(format string, v ...interface{}) {
	if sio.config.Logger != nil {
		sio.config.Logger.Printf(format, v...)
	}
}

// Handle is invoked on every http-request coming through the muxer.
// It is responsible for parsing the request and passing the http conn/req -pair
// to the corresponding sio connections. It also creates new connections when needed.
// The URL and method must be one of the following:
//
// OPTIONS *
//     GET resource
//     GET resource/sessionid
//    POST resource/sessionid
func (sio *SocketIO) handle(t Transport, w http.ResponseWriter, req *http.Request) {
	var parts []string
	var c *Conn
	var err error

	if !sio.isAuthorized(req) {
		sio.Log("sio/handle: unauthorized request:", req)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if origin := req.Header.Get("Origin"); origin != "" {
		if _, ok := sio.verifyOrigin(origin); !ok {
			sio.Log("sio/handle: unauthorized origin:", origin)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET")
	}

	switch req.Method {
	case "OPTIONS":
		w.WriteHeader(http.StatusOK)
		return

	case "GET", "POST":
		break

	default:
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// TODO: fails if the session id matches the transport
	if i := strings.LastIndex(req.URL.Path, t.Resource()); i >= 0 {
		pathLen := len(req.URL.Path)
		if req.URL.Path[pathLen-1] == '/' {
			pathLen--
		}

		parts = strings.Split(req.URL.Path[i:pathLen], "/")
	}

	if len(parts) < 2 || parts[1] == "" {
		c, err = newConn(sio)
		if err != nil {
			sio.Log("sio/handle: unable to create a new connection:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else {
		c = sio.GetConn(SessionID(parts[1]))
	}

	// we should now have a connection
	if c == nil {
		sio.Log("sio/handle: unable to map request to connection:", req.URL.Raw)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// pass the http conn/req pair to the connection
	if err = c.handle(t, w, req); err != nil {
		sio.Logf("sio/handle: conn/handle: %s: %s", c, err)
		w.WriteHeader(http.StatusUnauthorized)
	}
}

// OnConnect is invoked by a connection when a new connection has been
// established succesfully. The establised connection is passed as an
// argument. It stores the connection and calls the user's OnConnect callback.
func (sio *SocketIO) onConnect(c *Conn) {
	sio.sessionsLock.Lock()
	sio.sessions[c.sessionid] = c
	sio.sessionsLock.Unlock()

	if sio.callbacks.onConnect != nil {
		sio.callbacks.onConnect(c)
	}
}

// OnDisconnect is invoked by a connection when the connection is considered
// to be lost. It removes the connection and calls the user's OnDisconnect callback.
func (sio *SocketIO) onDisconnect(c *Conn) {
	sio.sessionsLock.Lock()
	delete(sio.sessions, c.sessionid)
	sio.sessionsLock.Unlock()

	if sio.callbacks.onDisconnect != nil {
		sio.callbacks.onDisconnect(c)
	}
}

// OnMessage is invoked by a connection when a new message arrives. It passes
// this message to the user's OnMessage callback.
func (sio *SocketIO) onMessage(c *Conn, msg Message) {
	if sio.callbacks.onMessage != nil {
		sio.callbacks.onMessage(c, msg)
	}
}

// isAuthorized is called during the handle() of any new http request
// If the user has set a callback, this is a hook for returning whether
// the connection is authorized. If no callback has been set, this method
// always returns true as a pass-through
func (sio *SocketIO) isAuthorized(req *http.Request) bool {
	if sio.callbacks.isAuthorized != nil {
		return sio.callbacks.isAuthorized(req)
	}
	return true
}

func (sio *SocketIO) verifyOrigin(reqOrigin string) (string, bool) {
	if sio.config.Origins == nil {
		return "", false
	}

	u, err := url.Parse(reqOrigin)
	if err != nil || u.Host == "" {
		return "", false
	}

	host := strings.SplitN(u.Host, ":", 2)

	for _, o := range sio.config.Origins {
		origin := strings.SplitN(o, ":", 2)
		if origin[0] == "*" || origin[0] == host[0] {
			if len(origin) < 2 || origin[1] == "*" {
				return o, true
			}
			if len(host) < 2 {
				switch u.Scheme {
				case "http", "ws":
					if origin[1] == "80" {
						return o, true
					}

				case "https", "wss":
					if origin[1] == "443" {
						return o, true
					}
				}
			} else if origin[1] == host[1] {
				return o, true
			}
		}
	}

	return "", false
}

func (sio *SocketIO) generatePolicyFile() []byte {
	buf := new(bytes.Buffer)
	buf.WriteString(`<?xml version="1.0"?>
<!DOCTYPE cross-domain-policy SYSTEM "http://www.macromedia.com/xml/dtds/cross-domain-policy.dtd">
<cross-domain-policy>
	<site-control permitted-cross-domain-policies="master-only" />
`)

	if sio.config.Origins != nil {
		for _, origin := range sio.config.Origins {
			parts := strings.SplitN(origin, ":", 2)
			if len(parts) < 1 {
				continue
			}
			host, port := "*", "*"
			if parts[0] != "" {
				host = parts[0]
			}
			if len(parts) == 2 && parts[1] != "" {
				port = parts[1]
			}

			fmt.Fprintf(buf, "\t<allow-access-from domain=\"%s\" to-ports=\"%s\" />\n", host, port)
		}
	}

	buf.WriteString("</cross-domain-policy>\n")
	return buf.Bytes()
}

func (sio *SocketIO) ListenAndServeFlashPolicy(laddr string) error {
	var listener net.Listener

	listener, err := net.Listen("tcp", laddr)
	if err != nil {
		return err
	}

	policy := sio.generatePolicyFile()

	for {
		conn, err := listener.Accept()
		if err != nil {
			sio.Log("ServeFlashsocketPolicy:", err)
			continue
		}

		go func() {
			defer conn.Close()

			buf := make([]byte, 20)
			if _, err := io.ReadFull(conn, buf); err != nil {
				sio.Log("ServeFlashsocketPolicy:", err)
				return
			}
			if !bytes.Equal([]byte("<policy-file-request"), buf) {
				sio.Logf("ServeFlashsocketPolicy: expected \"<policy-file-request\" but got %q", buf)
				return
			}

			var nw int
			for nw < len(policy) {
				n, err := conn.Write(policy[nw:])
				if err != nil && err != os.EAGAIN {
					sio.Log("ServeFlashsocketPolicy:", err)
					return
				}
				if n > 0 {
					nw += n
					continue
				} else {
					sio.Log("ServeFlashsocketPolicy: wrote 0 bytes")
					return
				}
			}
			sio.Log("ServeFlashsocketPolicy: served", conn.RemoteAddr())
		}()
	}

	return nil
}
