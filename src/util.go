package socketio

import (
	"log"
	"os"
)

type dummyWriter struct{}

func (dw dummyWriter) Write(p []byte) (n int, err os.Error) {
	return len(p), nil
}

var (
	DummyLogger   = log.New(dummyWriter{}, nil, "", log.Lok)
	DefaultLogger = log.New(os.Stdout, nil, "", log.Ldate|log.Ltime)
	DebugLogger   = log.New(os.Stdout, nil, "", log.Ldate|log.Ltime|log.Lshortfile)
)
