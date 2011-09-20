include $(GOROOT)/src/Make.inc

TARG = socketio
GOFILES = \
	client.go \
	codec.go \
	config.go \
	connection.go \
	flashpolicy.go \
	message.go \
	server.go \
	transport.go \
	transport_flashsocket.go \
	transport_htmlfile.go \
	transport_jsonppolling.go \
	transport_websocket.go \
	transport_xhrpolling.go \
	util.go

include $(GOROOT)/src/Make.pkg

.PHONY: gofmt
gofmt:
	gofmt -w $(GOFILES)
