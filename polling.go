package zmq4

/*
#include <zmq.h>
*/
import "C"

import (
	"fmt"
	"time"
)

// Return type for (*Poller)Poll
type Polled struct {
	SocketRef *SocketRef // socket with matched event(s)
	Events    State      // actual matched event(s)
}

type SocketRef struct {
	Index  int // public field to be used by the caller.
	socket *Socket
	idx    int // internal field to reference in items and socks slice.
}

func (sr *SocketRef) Socket() *Socket {
	return sr.socket
}

type Poller struct {
	items []C.zmq_pollitem_t
	socks []*SocketRef
}

// Create a new Poller
func NewPoller() *Poller {
	return &Poller{
		items: make([]C.zmq_pollitem_t, 0),
		socks: make([]*SocketRef, 0),
	}
}

// Add items to the poller
//
// Events is a bitwise OR of zmq.POLLIN and zmq.POLLOUT
//
// Returns the id of the item, which can be used as a handle to
// (*Poller)Update and as an index into the result of (*Poller)PollAll
func (p *Poller) Add(soc *Socket, events State) *SocketRef {
	var item C.zmq_pollitem_t
	item.socket = soc.soc
	item.fd = 0
	item.events = C.short(events)
	socref := &SocketRef{socket: soc, idx: len(p.items)}
	p.items = append(p.items, item)
	p.socks = append(p.socks, socref)
	return socref
}

// Update the events mask of a socket in the poller
//
// Replaces the Poller's bitmask for the specified id with the events parameter passed
//
// Returns the previous value, or ErrorNoSocket if the id was out of range
func (p *Poller) Update(soc *SocketRef, events State) (previous State, err error) {
	if soc.idx >= 0 && soc.idx < len(p.items) {
		previous = State(p.items[soc.idx].events)
		p.items[soc.idx].events = C.short(events)
		return previous, nil
	}
	return 0, ErrorNoSocket
}

// Remove a socket from the poller
//
// Returns ErrorNoSocket if the id was out of range
func (p *Poller) Remove(soc *SocketRef) error {
	if soc.idx >= 0 && soc.idx < len(p.items) {
		last := len(p.items) - 1
		if len(p.items) > 1 {
			p.items[soc.idx], p.items[last] = p.items[last], p.items[soc.idx]
			p.socks[soc.idx], p.socks[last] = p.socks[last], p.socks[soc.idx]
			p.socks[soc.idx].idx = soc.idx
		}
		p.items = p.items[:last]
		p.socks = p.socks[:last]
		return nil
	}
	return ErrorNoSocket
}

/*
Input/output multiplexing

If timeout < 0, wait forever until a matching event is detected

Only sockets with matching socket events are returned in the list.

Example:

    poller := zmq.NewPoller()
    poller.Add(socket0, zmq.POLLIN)
    poller.Add(socket1, zmq.POLLIN)
    //  Process messages from both sockets
    for {
        sockets, _ := poller.Poll(-1)
        for _, socket := range sockets {
            switch s := socket.Socket; s {
            case socket0:
                msg, _ := s.Recv(0)
                //  Process msg
            case socket1:
                msg, _ := s.Recv(0)
                //  Process msg
            }
        }
    }
*/
func (p *Poller) Poll(timeout time.Duration) ([]Polled, error) {
	return p.poll(timeout, false)
}

/*
This is like (*Poller)Poll, but it returns a list of all sockets,
in the same order as they were added to the poller,
not just those sockets that had an event.

For each socket in the list, you have to check the Events field
to see if there was actually an event.

When error is not nil, the return list contains no sockets.
*/
func (p *Poller) PollAll(timeout time.Duration) ([]Polled, error) {
	return p.poll(timeout, true)
}

func (p *Poller) poll(timeout time.Duration, all bool) ([]Polled, error) {
	lst := make([]Polled, 0, len(p.items))

	for _, soc := range p.socks {
		if !soc.socket.opened {
			return lst, ErrorSocketClosed
		}
	}

	t := timeout
	if t > 0 {
		t = t / time.Millisecond
	}
	if t < 0 {
		t = -1
	}
	rv, err := C.zmq_poll(&p.items[0], C.int(len(p.items)), C.long(t))
	if rv < 0 {
		return lst, errget(err)
	}
	for i, it := range p.items {
		if all || it.events&it.revents != 0 {
			lst = append(lst, Polled{p.socks[i], State(it.revents)})
		}
	}
	return lst, nil
}

// Poller as string.
func (p *Poller) String() string {
	str := make([]string, 0)
	for i, poll := range p.items {
		str = append(str, fmt.Sprintf("%v%v", p.socks[i], State(poll.events)))
	}
	return fmt.Sprint("Poller", str)
}
