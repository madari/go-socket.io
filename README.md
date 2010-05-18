go-socket.io
============

The `socketio` package is a simple abstraction layer for different web browser-
supported transport mechanisms. It is meant to be fully compatible with the
[Socket.IO client](http://github.com/LearnBoost/Socket.IO) JavaScript-library by
[LearnBoost Labs](http://socket.io/), but through custom formatters it should
suit for any client implementation.

It provides an easy way for developers to rapidly prototype with the most
popular browser transport mechanism today:

- [HTML5 WebSockets](http://dev.w3.org/html5/websockets/)
- [XHR Polling](http://en.wikipedia.org/wiki/Comet_%28programming%29#XMLHttpRequest_long_polling)
- [XHR Multipart Streaming](http://en.wikipedia.org/wiki/Comet_%28programming%29#XMLHttpRequest)

## Disclaimer

**The go-socket.io is still very experimental, and you should consider it as an
early prototype.** I hope it will generate some conversation and most
importantly get other contributors.

## Crash course

The `socketio` package works hand-in-hand with the standard `http` package (by
plugging itself into a configurable `http.ServeMux`) and hence it doesn't need a
full network port for itself. It has an callback-style event handling API. The
callbacks are:

- *socketio.OnConnect*
- *socketio.OnDisconnect*
- *socketio.OnMessage*

Other utility-methods include:

- *socketio.Mux*
- *socketio.Broadcast*
- *socketio.BroadcastExcept*
- *socketio.IterConns*
- *socketio.GetConn*

Each new connection will be automatically assigned an unique session id and
using those the clients can reconnect without losing messages: the socketio
package persists client's pending messages (until some configurable point) if
they can't be immediately delivered. All writes through the API are by design
asynchronous. For critical messages the package provides a way to detect
succesful deliveries by using `socketio.Conn.WaitFlush` *[TODO: better
solution]*. All-in-all, the `socketio.Conn` type has two methods to handle
message passing:

- *socketio.Conn.Send*
- *socketio.Conn.WaitFlush*

## Example: A simple chat server

	package main

	import (
		"http"
		"log"
		"socketio"
	)

	// A very simple chat server
	func main() {
		// create the server and mux it to /socket.io/ in http.DefaultServeMux
		sio := socketio.NewSocketIO(nil)
		sio.Mux("/socket.io/", nil)

		// serve static files under www/
		http.Handle("/", http.FileServer("www/", "/"))

		// client connected. Let everyone know about this.
		sio.OnConnect(func(c *socketio.Conn) {
			// socketio does not care what you are sending as long as it is
			// marshallable by the standard json-package
			sio.Broadcast(struct{ announcement string }{"connected: " + c.String()})
		})

		// client disconnected. Let the other users know about this.
		sio.OnDisconnect(func(c *socketio.Conn) {
			sio.BroadcastExcept(c,
				struct{ announcement string }{"disconnected: " + c.String()})
		})

		// client sent a message. Let's broadcast it to the other users.
		sio.OnMessage(func(c *socketio.Conn, msg string) {
			sio.BroadcastExcept(c,
				struct{ message []string }{[]string{c.String(), msg}})
		})

		// start serving
		log.Stdout("Server started.")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Stdout("ListenAndServe: %s", err.String())
			os.Exit(1)
		}
	}

## License 

(The MIT License)

Copyright (c) 2010 Jukka-Pekka Kekkonen &lt;karatepekka@gmail.com&gt;

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
