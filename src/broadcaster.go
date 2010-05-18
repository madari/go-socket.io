package socketio

import "container/list"

type Broadcaster struct {
	chans      *list.List
	Write      chan interface{}
	Register   chan *list.Element
	Unregister chan *list.Element
}

func NewBroadcaster() *Broadcaster {
	b := &Broadcaster{
		chans:      new(list.List),
		Write:      make(chan interface{}),
		Register:   make(chan *list.Element),
		Unregister: make(chan *list.Element),
	}

	go func() {
		nextnode := b.chans.PushBack(make(chan interface{}))

		for {
			select {
			case b.Register <- nextnode:
				nextnode = b.chans.PushBack(make(chan interface{}))

			case el := <-b.Unregister:
				b.chans.Remove(el)
				close(el.Value.(chan interface{}))

			case data := <-b.Write:
				// iterate, but discard the last element
				for el := b.chans.Front(); el != b.chans.Back(); el = el.Next() {
					el.Value.(chan interface{}) <- data
				}
			}
		}
	}()

	return b
}
