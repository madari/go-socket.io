package socketio

import (
	"log"
	"os"
)

type nopWriter struct{}

func (nw nopWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

var (
	NOPLogger     = log.New(nopWriter{}, "", 0)
	DefaultLogger = log.New(os.Stdout, "", log.Ldate|log.Ltime)
)
