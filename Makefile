include $(GOROOT)/src/Make.inc

TARG = socketio
GOFILES = \
	util.go \
	servemux.go \
	message.go \
	config.go \
	session.go \
	socketio.go \
	connection.go \
	codec.go \
	codec_sio.go \
	codec_siostreaming.go \
	transport.go \
	transport_xhrpolling.go \
	transport_xhrmultipart.go \
	transport_htmlfile.go \
	transport_websocket.go \
	transport_flashsocket.go \
	transport_jsonppolling.go \
	client.go \
	doc.go \
	
include $(GOROOT)/src/Make.pkg

.PHONY: gofmt
gofmt:
	gofmt -w $(GOFILES)
