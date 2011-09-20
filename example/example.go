package main

import (
	"http"
	"log"
	"socketio"
	"sync"
	"time"
)

var (
	nicks = make(map[string]string)
	mu    sync.Mutex
)

func main() {
	// use verbose logger
	socketio.Log = socketio.VerboseLogger
	sio := socketio.NewServer(nil)

	http.Handle("/", http.FileServer(http.Dir("www/")))
	http.Handle("/socket.io/", http.StripPrefix("/socket.io/", sio.Handler(func(c *socketio.Conn) {
		var msg socketio.Message
		var nick string

		for {
			if err := c.Receive(&msg); err != nil {
				break
			}

			event, _ := msg.Event()
			switch event {
			case "nickname":
				if err := msg.ReadArguments(&nick); err != nil {
					continue
				}

				mu.Lock()
				if nicks[nick] != "" {
					c.Reply(&msg, true)
				} else {
					c.Reply(&msg, false)
					nicks[nick] = nick
					sio.EmitExcept(c, "announcement", nick+" connected")
					sio.Emit("nicknames", nicks)
				}
				mu.Unlock()

			case "user message":
				var payload string
				if err := msg.ReadArguments(&payload); err != nil {
					continue
				}
				sio.EmitExcept(c, "user message", nick, payload)
			}
		}

		if nick != "" {
			mu.Lock()
			nicks[nick] = "", false
			sio.Emit("announcement", nick+" disconnected")
			sio.Emit("nicknames", nicks)
			mu.Unlock()
		}
	})))

	go func() {
		time.Sleep(2e9)
		log.Println("Spawning echo client")
		echo()
	}()

	log.Println("Server starting. Tune your browser to http://localhost:8080/")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}

func echo() {
	var msg socketio.Message
	c, err := socketio.Dial("http://localhost:8080/socket.io/", "http://localhost:8080/")
	defer c.Close()
	if err != nil {
		panic(err)
	}
	c.Emit("nickname", "echo-client")
	for {
		if err := c.Receive(&msg); err != nil {
			log.Print("client received error: ", err)
			return
		}
		c.Emit("user message", msg.String())
	}
}
