go-socket.io
============

The `socketio` package is a simple abstraction layer for different web browser-
supported transport mechanisms. It is fully compatible with the
[Socket.IO client](http://github.com/LearnBoost/Socket.IO) (version 0.6) JavaScript-library by
[LearnBoost Labs](http://socket.io/). By writing custom codecs the `socketio`
could be perhaps used with other clients, too.

It provides an easy way for developers to rapidly prototype with the most
popular browser transport mechanism today:

- [HTML5 WebSockets](http://dev.w3.org/html5/websockets/)
- [Adobe® Flash® Sockets](https://github.com/gimite/web-socket-js)
- [JSONP Long Polling](http://en.wikipedia.org/wiki/JSONP#JSONP)
- [XHR Long Polling](http://en.wikipedia.org/wiki/Comet_%28programming%29#XMLHttpRequest_long_polling)
- [XHR Multipart Streaming](http://en.wikipedia.org/wiki/Comet_%28programming%29#XMLHttpRequest)
- [ActiveX HTMLFile](http://cometdaily.com/2007/10/25/http-streaming-and-internet-explorer/)

## Compatibility with Socket.IO 0.7->

**Go-socket.io is currently compatible with Socket.IO 0.6 clients only.**

## Demo

[The Paper Experiment](http://wall-r.com/paper)

## Crash course

The `socketio` package works hand-in-hand with the standard `http` package (by
plugging itself into `http.ServeMux`) and hence it doesn't need a
full network port for itself. It has an callback-style event handling API. The
callbacks are:

- *SocketIO.OnConnect*
- *SocketIO.OnDisconnect*
- *SocketIO.OnMessage*

Other utility-methods include:

- *SocketIO.ServeMux*
- *SocketIO.Broadcast*
- *SocketIO.BroadcastExcept*
- *SocketIO.GetConn*

Each new connection will be automatically assigned an session id and
using those the clients can reconnect without losing messages: the server
persists clients' pending messages (until some configurable point) if they can't
be immediately delivered. All writes are by design asynchronous and can be made
through `Conn.Send`. The server also abstracts handshaking and various keep-alive mechanisms.

Finally, the actual format on the wire is described by a separate `Codec`. The
default bundled codecs, `SIOCodec` and `SIOStreamingCodec` are fully compatible
with the LearnBoost's [Socket.IO client](http://github.com/LearnBoost/Socket.IO)
(master and development branches).

## Example: A simple chat server

	package main

	import (
		"http"
		"log"
		"socketio"
	)

	func main() {
		sio := socketio.NewSocketIO(nil)

		sio.OnConnect(func(c *socketio.Conn) {
			sio.Broadcast(struct{ announcement string }{"connected: " + c.String()})
		})

		sio.OnDisconnect(func(c *socketio.Conn) {
			sio.BroadcastExcept(c,
				struct{ announcement string }{"disconnected: " + c.String()})
		})

		sio.OnMessage(func(c *socketio.Conn, msg socketio.Message) {
			sio.BroadcastExcept(c,
				struct{ message []string }{[]string{c.String(), msg.Data()}})
		})

		mux := sio.ServeMux()
		mux.Handle("/", http.FileServer("www/", "/"))

		if err := http.ListenAndServe(":8080", mux); err != nil {
			log.Fatal("ListenAndServe:", err)
		}
	}

## tl;dr

You can get the code and run the bundled example by following these steps:

	$ git clone git://github.com/madari/go-socket.io.git
	$ cd go-socket.io
	$ git submodule update --init --recursive
	$ make install
	$ cd example
	$ make
	$ ./example

## License 

(The MIT License)

Copyright (c) 2011 Jukka-Pekka Kekkonen &lt;karatepekka@gmail.com&gt;

Permission is hereby granted, free of charge, to any person obtaining
a copy of this software and associated documentation files (the
'Software'), to deal in the Software without restriction, including
without limitation the rights to use, copy, modify, merge, publish,
distribute, sublicense, and/or sell copies of the Software, and to
permit persons to whom the Software is furnished to do so, subject to
the following conditions:

The above copyright notice and this permission notice shall be
included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED 'AS IS', WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY
CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,
TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
