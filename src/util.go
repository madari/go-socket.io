package socketio

import (
	"log"
	"os"
	"syscall"
	"sync"
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

// dMutex is a silly wrapper for sync.Mutex that gives some verbose
// information about their usage and helps debugging. The goal is to replace
// all mutexes with go-style inter-thread communicating at some point
// and hence this is a really temporarily solution.
type dMutex struct {
	mutex  *sync.Mutex
	name   string
	locked bool
	owner  string
}

func newDMutex(name string) (dm *dMutex) {
	dm = &dMutex{
		mutex: new(sync.Mutex),
		name:  name,
		owner: "-",
	}

	go func() {
		syscall.Sleep(10e9)
		log.Stdoutf("DMutex/%s: status => %v", dm.name, dm.locked)
	}()

	return
}

func (dm *dMutex) Lock(acquirer string) {
	//log.Stdoutf("DMutex/%s[%s]: aquiring lock (owner: %s)", dm.name, acquirer, dm.owner)
	dm.mutex.Lock()
	dm.owner = acquirer
	dm.locked = true
	//log.Stdoutf("DMutex/%s[%s]: lock aquired", dm.name, acquirer)
}

func (dm *dMutex) Unlock() {
	//log.Stdoutf("DMutex/%s[%s]: releasing lock", dm.name, dm.owner)
	dm.mutex.Unlock()
	dm.locked = false
	dm.owner = "-"
}
