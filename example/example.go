package main

import (
	"container/vector"
	"http"
	"log"
	"os"
	"socketio"
	"sync"
)

// A very simple chat server
func main() {
	buffer := new(vector.Vector)
	mutex := new(sync.Mutex)

	// Create the socket.io server and mux it to /socket.io/
	sio := socketio.NewSocketIO(nil)
	sio.Mux("/socket.io/", nil)

	// Client connected
	// Send the buffer to the client and broadcast announcement
	sio.OnConnect(func(c *socketio.Conn) {
		mutex.Lock()
		c.Send(struct{ buffer []interface{} }{buffer.Data()})
		mutex.Unlock()

		sio.Broadcast(struct{ announcement string }{"connected: " + c.String()})
	})

	// Client disconnected
	// Send the announcement
	sio.OnDisconnect(func(c *socketio.Conn) {
		sio.Broadcast(struct{ announcement string }{"disconnected: " + c.String()})
	})

	// Client sent a message
	// Store it to the buffer and broadcast it
	sio.OnMessage(func(c *socketio.Conn, msg string) {
		payload := struct{ message []string }{[]string{c.String(), msg}}

		mutex.Lock()
		buffer.Push(payload)
		mutex.Unlock()

		sio.BroadcastExcept(c, payload)
	})
	
	// Server static files under www/
	http.Handle("/", http.FileServer("www/", "/"))

	log.Stdout("Server starting. Tune your browser to http://localhost:8080/")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Stdout("ListenAndServe: ", err.String())
		os.Exit(1)
	}
}
