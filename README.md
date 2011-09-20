go-socket.io
============

The `socketio` package is an transport abstraction layer attempting to make realtime
web apps possible on every browser. It is meant to be used with the
[Socket.IO client](http://github.com/LearnBoost/socket.io-client) JavaScript-library by
[LearnBoost Labs](http://socket.io/).

Socket.IO uses feature detection to pick the best possible transport in every situation
while abstracting all of this from the developer. Depending on the case, it chooses
one of the following transports:

- [HTML5 WebSockets](http://dev.w3.org/html5/websockets/)
- [Adobe® Flash® Sockets](https://github.com/gimite/web-socket-js)
- [JSONP Long Polling](http://en.wikipedia.org/wiki/JSONP#JSONP)
- [XHR Long Polling](http://en.wikipedia.org/wiki/Comet_%28programming%29#XMLHttpRequest_long_polling)
- [ActiveX HTMLFile](http://cometdaily.com/2007/10/25/http-streaming-and-internet-explorer/)

New connections are assigned an session id and using those the clients can reconnect
without losing messages: the server
persists clients' pending messages if they can't
be immediately delivered. 

## Crash course

The `socketio` package works hand-in-hand with the standard `http` package and hence
must be muxed like any other `http.Handler`:

```go
sio := socketio.NewServer(nil)
http.Handle("/socket.io/", http.StripPrefix("/socket.io/", 
	sio.Handler(func (c *socketio.Conn) { /* whee */ })
))
```

After a connection has been established, the handler will be invoked
and the connection will be terminated when the handler returns:

```go
func (c *socketio.Conn) {
	// I shall disconnect after 10 seconds
	time.Sleep(10e9)
})
```

### Receiving messages

The connection exposes `Receive(*Message)` method for receiving data from the
server. The method will block until the connection is shutdown or a message
was received. The connection will internally handle various "side-channel"
messages while `Receive `is blocking, and hence to keep the connection healthy,
one should always loop around Receive until an `os.EOF` is returned:

```go
func (c *socketio.Conn) {
	var msg socketio.Message
	for {
		// I'm a healthy connection, yay
		if err := c.Receive(&msg); err != nil {
			return
		}
	}
})
```

The Socket.IO protocol describes to different kind of messages: normal messages and
so called events. Normal messages contain textual payload, where events contain a
name and arguments. One could think of events as remote procedure calls.
When receiving messages, one can use the following pattern to
determine what the messages actually is:

```go
func (c *socketio.Conn) {
	var msg socketio.Message
	for {
		if err := c.Receive(&msg); err != nil {
			return
		}
		switch msg.Type() {
		case socketio.MessageJSON, socketio.MessageText:
			fmt.Println("this is an ordinary message with payload: ", msg.String())
		case socketio.MessageEvent:
			var arg0 string
			var arg1 int
			name, _ := msg.Event()
			msg.ReadArguments(&arg0, &arg1)
			fmt.Printf("this is an event with name %s. Arguments are arg0=%s, arg1=%d"
				name, arg0, arg1)
		}
	}
})
```

### Sending messages

To send messages, the connection exposes two different methods for normal messages and events
respectively: `Send(interface{})` and `Emit(string, ...interface{})`. Both calls are always
asynchronous and simply queue the payload to be dispatched when the next suitable moments arrives.
If the payload is an `string` or a `[]byte` the message will be tagged as `MessageText`, otherwise
the payload will be marshalled into JSON and tagged as `MessageJSON`. For example:

```go
func (c *socketio.Conn) {
	c.Send("text message")
	c.Send(struct{X string}{"json message"})
	c.Emit("myevent")
	c.Emit("myevent", "1st argument", 2, struct{X string}{"third argument"})
	for {
		if err := c.Receive(&msg); err != nil {
			return
		}
	}
})
```

### Acknowledging message

The protocol also describes acknowledgements. The need for acknowledgements is
application specific: the sender defines this on per message basis. If the client
expects an acknowledgement, the message can be acknowledged using the `Reply(*Message, ...interface{})`
method, which has the same semantics as `Emit`.

```go
func (c *socketio.Conn) {
	for {
		if err := c.Receive(&msg); err != nil {
			return
		}
		c.Reply(&msg, "1st argument", 2)
	}
})
```

## Concrete example: A simple chat server

```go
package main

import (
	"http"
	"socketio"
)

func main() {
	sio := socketio.NewServer(nil)
	http.Handle("/socket.io/", http.StripPrefix("/socket.io",
		sio.Handler(func(c *socketio.Conn) {
			sio.Broadcast("connected: " + c.String())
			defer sio.Broadcast("disconnected: " + c.String())
			for {
				if err := c.Receive(&msg); err != nil {
					return
				}
				sio.BroadcastExcept(c, msg.String())
			}
		})
	))
	if err := http.ListenAndServe(":80", nil); err != nil {
		panic(err)
	}
}
```

## Getting the code

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
