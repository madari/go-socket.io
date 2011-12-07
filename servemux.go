package socketio

import (
	"net/http"
	"strings"
)

type ServeMux struct {
	*http.ServeMux
	sio *SocketIO
}

func NewServeMux(sio *SocketIO) *ServeMux {
	return &ServeMux{http.NewServeMux(), sio}
}

func (mux *ServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, mux.sio.config.Resource) {
		rest := r.URL.Path[len(mux.sio.config.Resource):]
		n := strings.Index(rest, "/")
		if n < 0 {
			n = len(rest)
		}
		if t, ok := mux.sio.transportLookup[rest[:n]]; ok {
			mux.sio.handle(t, w, r)
			return
		}
	}

	mux.ServeMux.ServeHTTP(w, r)
}
