package socketio

import (
	"crypto/rand"
	"io"
	"log"
	"os"
)

const (
	sessionIdLength  = 16
	sessionIdCharset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

var (
	Log           = DefaultLogger
	VerboseLogger = &Logger{debugLogger, infoLogger, warnLogger}
	DefaultLogger = &Logger{nil, infoLogger, warnLogger}

	debugLogger = log.New(os.Stderr, "[DEBUG] ", log.Ldate|log.Ltime)
	infoLogger  = log.New(os.Stderr, "[INFO]  ", log.Ldate|log.Ltime)
	warnLogger  = log.New(os.Stderr, "[WARN]  ", log.Ldate|log.Ltime)
)

type Logger struct {
	Debug, Info, Warn *log.Logger
}

func (l *Logger) debug(v ...interface{}) {
	if l.Debug != nil {
		l.Debug.Print(v...)
	}
}

func (l *Logger) debugf(format string, v ...interface{}) {
	if l.Debug != nil {
		l.Debug.Printf(format, v...)
	}
}

func (l *Logger) info(v ...interface{}) {
	if l.Info != nil {
		l.Info.Print(v...)
	}
}

func (l *Logger) infof(format string, v ...interface{}) {
	if l.Info != nil {
		l.Info.Printf(format, v...)
	}
}

func (l *Logger) warn(v ...interface{}) {
	if l.Warn != nil {
		l.Warn.Print(v...)
	}
}

func (l *Logger) warnf(format string, v ...interface{}) {
	if l.Warn != nil {
		l.Warn.Printf(format, v...)
	}
}

func newSessionId() (sid string, err error) {
	b := make([]byte, sessionIdLength)
	if _, err = io.ReadFull(rand.Reader, b); err != nil {
		return
	}
	for i := 0; i < sessionIdLength; i++ {
		b[i] = sessionIdCharset[b[i]%uint8(len(sessionIdCharset))]
	}
	sid = string(b)
	return
}
