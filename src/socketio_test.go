package socketio

import (
	"http"
	"testing"
	"time"
	"fmt"
)

const (
	serverAddr = "127.0.0.1:6060"

	eventConnect = iota
	eventDisconnect
	eventMessage
	eventCrash
)

var events chan *event
var server *SocketIO

type event struct {
	conn      *Conn
	eventType uint8
	msg       Message
}

func echoServer(addr string, config *Config) <-chan *event {
	events := make(chan *event)

	server = NewSocketIO(config)
	server.OnConnect(func(c *Conn) {
		events <- &event{c, eventConnect, nil}
	})
	server.OnDisconnect(func(c *Conn) {
		events <- &event{c, eventDisconnect, nil}
	})
	server.OnMessage(func(c *Conn, msg Message) {
		c.Send(msg.Data())
		events <- &event{c, eventMessage, msg}
	})
	server.Mux("/socket.io/", nil)

	go func() {
		http.ListenAndServe(addr, nil)
		events <- &event{nil, eventCrash, nil}
	}()

	return events
}


func TestWebsocket(t *testing.T) {
	finished := make(chan bool, 1)
	clientMessage := make(chan Message)
	clientDisconnect := make(chan bool)
	numMessages := 313

	config := DefaultConfig
	config.Origins = []string{serverAddr}
	serverEvents := echoServer(serverAddr, &config)

	client := NewWebsocketClient(SIOCodec{})
	client.OnMessage(func(msg Message) {
		clientMessage <- msg
	})
	client.OnDisconnect(func() {
		clientDisconnect <- true
	})

	time.Sleep(1e9)
	/*
		go func() {
			time.Sleep(5e9)
			if _, ok := <-finished; !ok {
				t.Fatalf("timeout")
			}
		}()*/

	err := client.Dial("ws://"+serverAddr+"/socket.io/websocket", "http://"+serverAddr+"/")
	if err != nil {
		t.Fatal(err)
	}

	// expect connection
	serverEvent := <-serverEvents
	if serverEvent.eventType != eventConnect || serverEvent.conn.sessionid != client.SessionID() {
		t.Fatalf("Expected eventConnect but got %v", serverEvent)
	}

	iook := make(chan bool)

	go func() {
		for i := 0; i < numMessages; i++ {
			t.Logf("Client sending %d/%d", i+1, numMessages)
			if err = client.Send(i); err != nil {
				t.Fatal("Send:", err)
			}
		}
		iook <- true
	}()

	go func() {
		for i := 0; i < numMessages; i++ {
			serverEvent = <-serverEvents
			t.Logf("Server event %q", serverEvent)

			expect := fmt.Sprintf("%d", i)
			if serverEvent.eventType != eventMessage || serverEvent.conn.sessionid != client.SessionID() {
				t.Fatalf("Expected eventMessage but got %#v", serverEvent)
			}
			if serverEvent.msg.Data() != expect {
				t.Fatalf("Server expected %s but received %s", expect, serverEvent.msg.Data())
			}
		}
		iook <- true
	}()

	go func() {
		for i := 0; i < numMessages; i++ {
			msg := <-clientMessage
			t.Log("Client received", msg.Data())
			expect := fmt.Sprintf("%d", i)
			if msg.Data() != expect {
				t.Fatalf("Client expected %s but received %s", expect, msg.Data())
			}
		}
		iook <- true
	}()

	for i := 0; i < 3; i++ {
		<-iook
	}

	go func() {
		if err = client.Close(); err != nil {
			t.Fatal("Close:", err)
		}
	}()

	t.Log("Waiting for client disconnect")
	<-clientDisconnect

	t.Log("Waiting for server disconnect")
	serverEvent = <-serverEvents
	if serverEvent.eventType != eventDisconnect || serverEvent.conn.sessionid != client.SessionID() {
		t.Fatalf("Expected disconnect event, but got %q", serverEvent)
	}

	finished <- true
}
