package main

import (
	"container/vector"
	"http"
	"log"
	"socketio"
	"sync"
)

// A very simple chat server
func main() {
	buffer := new(vector.Vector)
	mutex := new(sync.Mutex)
	
	// create the socket.io server and mux it to /socket.io/
	config := socketio.DefaultConfig
	config.Origins = []string{"localhost:8080"}
	sio := socketio.NewSocketIO(&config)
	
	// when a client connects - send it the buffer and broadcasta an announcement
	sio.OnConnect(func(c *socketio.Conn) {
		mutex.Lock()
		c.Send(struct{ buffer []interface{} }{buffer.Copy()})
		mutex.Unlock()
		sio.Broadcast(struct{ announcement string }{"connected: " + c.String()})
	})

	// when a client disconnects - send an announcement
	sio.OnDisconnect(func(c *socketio.Conn) {
		sio.Broadcast(struct{ announcement string }{"disconnected: " + c.String()})
	})

	// when a client send a message - broadcast and store it
	sio.OnMessage(func(c *socketio.Conn, msg socketio.Message) {
		payload := struct{ message []string }{[]string{c.String(), msg.Data()}}
		mutex.Lock()
		buffer.Push(payload)
		mutex.Unlock()
		sio.Broadcast(payload)
	})

	log.Println("Server starting. Tune your browser to http://localhost:8080/")

	sio.Mux("/socket.io/", nil)
	http.Handle("/", http.FileServer("www/", "/"))

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Exit("ListenAndServe:", err)
	}
}
