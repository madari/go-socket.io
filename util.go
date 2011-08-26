package socketio

import (
	"log"
	"os"
	"time"
	"sync"
)

type nopWriter struct{}

func (nw nopWriter) Write(p []byte) (n int, err os.Error) {
	return len(p), nil
}

var (
	NOPLogger     = log.New(nopWriter{}, "", 0)
	DefaultLogger = log.New(os.Stdout, "", log.Ldate|log.Ltime)
)

type DelayTimer struct {
	mu       sync.Mutex
	handling bool
	deadline int64
	Timeouts chan int64
	timer    *time.Timer
}

func NewDelayTimer() *DelayTimer {
	return &DelayTimer{Timeouts: make(chan int64)}
}

func (w *DelayTimer) Stop() {
	w.mu.Lock()
	if w.timer != nil {
		w.timer.Stop()
	}
	w.mu.Unlock()
}

func (w *DelayTimer) Reset(t int64) {
	t += time.Nanoseconds()
	w.mu.Lock()
	if t <= w.deadline {
		w.mu.Unlock()
		return
	}
	if w.timer != nil {
		w.timer.Stop()
	}
	w.timer = time.AfterFunc(t-time.Nanoseconds(), func() {
		// If previous timeout is still being handled, then 
		// ignore this timeout. 
		w.mu.Lock()
		if w.handling {
			w.mu.Unlock()
			return
		}
		w.handling = true
		w.mu.Unlock()
		w.Timeouts <- time.Nanoseconds()
		w.mu.Lock()
		w.handling = false
		w.mu.Unlock()
	})
	w.deadline = t
	w.mu.Unlock()
}
