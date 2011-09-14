package main

import (
	"container/vector"
	"http"
	"log"
	"socketio"
	"sync"
)

type Announcement struct {
	Announcement string `json:"announcement"`
}

type Buffer struct {
	Buffer []interface{} `json:"buffer"`
}

type Message struct {
	Message []string `json:"message"`
}

// A very simple chat server
func main() {
	buffer := new(vector.Vector)
	mutex := new(sync.Mutex)

	// create the socket.io server and mux it to /socket.io/
	config := socketio.DefaultConfig
	config.Origins = []string{"localhost:8080"}
	sio := socketio.NewSocketIO(&config)

	go func() {
		if err := sio.ListenAndServeFlashPolicy(":843"); err != nil {
			log.Println(err)
		}
	}()

	// when a client connects - send it the buffer and broadcasta an announcement
	sio.OnConnect(func(c *socketio.Conn) {
		mutex.Lock()
		c.Send(Buffer{buffer.Copy()})
		mutex.Unlock()
		sio.Broadcast(Announcement{"connected: " + c.String()})
	})

	// when a client disconnects - send an announcement
	sio.OnDisconnect(func(c *socketio.Conn) {
		sio.Broadcast(Announcement{"disconnected: " + c.String()})
	})

	// when a client send a message - broadcast and store it
	sio.OnMessage(func(c *socketio.Conn, msg socketio.Message) {
		payload := Message{[]string{c.String(), msg.Data()}}
		mutex.Lock()
		buffer.Push(payload)
		mutex.Unlock()
		sio.Broadcast(payload)
	})

	log.Println("Server starting. Tune your browser to http://localhost:8080/")

	mux := sio.ServeMux()
	mux.Handle("/", http.FileServer(http.Dir("www/")))

	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
