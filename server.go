package socketio

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

const (
	ProtocolVersion = 1
)

type server struct {
	config         *Config
	mutex          sync.RWMutex
	sessions       map[string]*Conn
	transportNames string
	transports     map[string]*Transport
}

func NewServer(config *Config) (s *server) {
	if config == nil {
		config = &DefaultConfig
	}

	s = &server{
		config:     config,
		sessions:   make(map[string]*Conn),
		transports: make(map[string]*Transport),
	}

	var names []string
	for _, t := range config.Transports {
		s.transports[t.Name] = t
		names = append(names, t.Name)
	}
	s.transportNames = strings.Join(names, ",")
	Log.debug("registered transports: ", s.transportNames)

	return
}

func (s *server) EmitExcept(c *Conn, name string, args ...interface{}) {
	s.mutex.RLock()
	for _, v := range s.sessions {
		if v != c {
			v.Emit(name, args...)
		}
	}
	s.mutex.RUnlock()
}

func (s *server) Emit(name string, args ...interface{}) {
	s.EmitExcept(nil, name, args...)
}

func (s *server) Broadcast(data interface{}) {
	s.BroadcastExcept(nil, data)
}

func (s *server) BroadcastExcept(c *Conn, data interface{}) {
	s.mutex.RLock()
	for _, v := range s.sessions {
		if v != c {
			v.Send(data)
		}
	}
	s.mutex.RUnlock()
}

func (s *server) AuthorizedHandler(a func(*http.Request) bool, f func(c *Conn)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if a(req) {
			s.serve(w, req, f)
			return
		}
		Log.warnf("%d %s %s unauthorized", http.StatusForbidden, req.Method, req.URL)
		w.WriteHeader(http.StatusForbidden)
	})
}

func (s *server) Handler(f func(c *Conn)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		s.serve(w, req, f)
	})
}

func (s *server) serve(w http.ResponseWriter, req *http.Request, f func(c *Conn)) {
	path := strings.TrimRight(req.URL.Path, "/")
	parts := strings.SplitN(path, "/", 3)
	l := len(parts)

	if l != 1 && l != 3 {
		Log.warnf("%d %s %s invalid path", http.StatusBadRequest, req.Method, req.URL)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if protocol, _ := strconv.Atoi(parts[0]); protocol != ProtocolVersion {
		Log.warnf("%d %s %s protocol version not supported: %s", http.StatusServiceUnavailable, req.Method, req.URL, parts[0])
		http.Error(w, "Protocol version not supported.", http.StatusServiceUnavailable)
		return
	}

	switch len(parts) {
	case 1:
		c, err := newConn(s)
		if err != nil {
			Log.warnf("%d %s %s unable to create new connection: %s", http.StatusInternalServerError, req.Method, req.URL, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		s.mutex.Lock()
		if s.sessions[c.sid] != nil {
			Log.warnf("%d %s %s session id collision", http.StatusInternalServerError, req.Method, req.URL, err)
			w.WriteHeader(http.StatusInternalServerError)
			s.mutex.Unlock()
			return
		}
		s.sessions[c.sid] = c
		s.mutex.Unlock()

		fmt.Fprintf(w, "%s:%d:%d:%s", c.sid, int(s.config.HeartbeatTimeout/1e9), int(s.config.CloseTimeout/1e9), s.transportNames)

		Log.infof("%d %s %s client %s connected", http.StatusOK, req.Method, req.URL, c)

		go func() {
			f(c)
			c.close()
			Log.infof("%d %s %s client %s disconnected", http.StatusOK, req.Method, req.URL, c)
		}()

		return
	case 3:
		transport := parts[1]
		sid := parts[2]

		var c *Conn
		var t *Transport

		if t = s.transports[transport]; t == nil {
			Log.warnf("%d %s %s unknown transport: %s", http.StatusServiceUnavailable, req.Method, req.URL, transport)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		s.mutex.Lock()
		if c = s.sessions[sid]; c == nil {
			Log.warnf("%d %s %s bad session id: %s", http.StatusInternalServerError, req.Method, req.URL, sid)
			w.WriteHeader(http.StatusInternalServerError)
			s.mutex.Unlock()
			return
		}
		s.mutex.Unlock()

		Log.debugf("%d %s %s client %s opening transport %s", http.StatusOK, req.Method, req.URL, c, transport)
		if err := c.handle(t, w, req); err != nil {
			Log.warnf("%d %s %s client %s unable to open transport %s: %s", http.StatusServiceUnavailable, req.Method, req.URL, c, transport, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		Log.debugf("%d %s %s client %s closed transport %s", http.StatusOK, req.Method, req.URL, c, transport)
	}
}
