package handlers

import (
	"context"
	"iter"
	"math/rand/v2"
	"sync"
	"time"

	// pwruntime rather than pw, because this file has no build tag: a source
	// names no transport, so both builds compile it, and only one of them has
	// pw. pwruntime.NamedSignal is pw.NamedSignal under its other name.
	"github.com/shibukawa/popcornweb/pwruntime"
)

// LoadRoomTitle is an ordinary async external: it answers once, and the
// boundary that binds it holds that value for the life of the render. It shares
// its clause with a live source below, which is legal and useful — the title is
// fetched once and the messages keep arriving.
func LoadRoomTitle(ctx context.Context, room string) (string, error) {
	select {
	case <-time.After(200 * time.Millisecond):
	case <-ctx.Done():
		return "", ctx.Err()
	}
	return "#" + room, nil
}

// WatchThroughput is a timer-paced live source: it decides when a new value
// exists, and the screen finds out because the server tells it.
//
// The context is not optional here. A source that never ends has nothing else
// to make it return, so without one this goroutine would outlive every reader.
func WatchThroughput(ctx context.Context) iter.Seq2[Sample, error] {
	return func(yield func(Sample, error) bool) {
		value := 40
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				value += rand.IntN(9) - 4
				if value < 1 {
					value = 1
				}
				sample := Sample{Label: "requests/s", Value: value, At: now.Format("15:04:05")}
				// Yielding blocks until this boundary is ready for another
				// value, so a source faster than the screen misses ticks
				// instead of filling a queue.
				if !yield(sample, nil) {
					return
				}
			}
		}
	}
}

// WatchMessages is an event-paced live source over a room many readers share.
//
// It yields the whole current list rather than the message that arrived. That
// is what makes a reconnect need no replay: the next delivery is sufficient on
// its own, so a gap costs freshness while it lasts and nothing afterwards.
func WatchMessages(ctx context.Context, name string) iter.Seq2[[]Message, error] {
	return func(yield func([]Message, error) bool) {
		// One upstream feeds every reader of this room. The framework renders
		// per client and owns no fan-out topology, so sharing a subscription
		// across clients is the source's job — and this is where it belongs.
		changed, leave := rooms.join(name)
		defer leave()
		if !yield(rooms.snapshot(name), nil) {
			return
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-changed:
				if !yield(rooms.snapshot(name), nil) {
					return
				}
				// And a signal saying one arrived.
				//
				// The delivery above cannot say this. It carries the whole list,
				// so a reader who was away for a minute gets the current room
				// and no notion of how much of it is new — which is the same
				// property that makes a reconnect cheap. Arrival is a different
				// fact from state, and it is the one the panel reacts to.
				//
				// It is yielded in the error slot and is not an error. The
				// runtime classifies it first, so nothing renders, no recover
				// subtree appears, and this subscription keeps running.
				//
				// No payload, deliberately. The handler needs to know that a
				// message arrived and nothing else, so this lets the server say
				// when and never what — which is the narrowest thing a
				// registered name can be given.
				if !yield(nil, pwruntime.NamedSignal("app.message")) {
					return
				}
			}
		}
	}
}

// rooms stands in for whatever a real application already has: a chat service,
// a pub/sub topic, a change feed.
var rooms = &roomSet{rooms: map[string]*room{}}

type roomSet struct {
	mutex sync.Mutex
	rooms map[string]*room
}

type room struct {
	mutex     sync.Mutex
	messages  []Message
	listeners map[chan struct{}]struct{}
	started   bool
	stop      chan struct{}
}

func (s *roomSet) get(name string) *room {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	existing, found := s.rooms[name]
	if !found {
		existing = &room{listeners: map[chan struct{}]struct{}{}, stop: make(chan struct{})}
		s.rooms[name] = existing
	}
	return existing
}

func (s *roomSet) snapshot(name string) []Message {
	return s.get(name).snapshot()
}

// join returns a channel that reports that something changed, and the function
// that unsubscribes. The channel is buffered by one and written without
// blocking, so a reader that is busy rendering coalesces the changes it missed
// into the one snapshot it takes next.
func (s *roomSet) join(name string) (<-chan struct{}, func()) {
	return s.get(name).join()
}

func (r *room) join() (<-chan struct{}, func()) {
	listener := make(chan struct{}, 1)
	r.mutex.Lock()
	r.listeners[listener] = struct{}{}
	if !r.started {
		r.started = true
		go r.chatter()
	}
	r.mutex.Unlock()
	return listener, func() {
		r.mutex.Lock()
		defer r.mutex.Unlock()
		delete(r.listeners, listener)
	}
}

func (r *room) snapshot() []Message {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return append([]Message(nil), r.messages...)
}

func (r *room) post(message Message) {
	r.mutex.Lock()
	r.messages = append(r.messages, message)
	if len(r.messages) > 8 {
		r.messages = r.messages[len(r.messages)-8:]
	}
	for listener := range r.listeners {
		select {
		case listener <- struct{}{}:
		default:
		}
	}
	r.mutex.Unlock()
}

var chatterLines = []struct{ author, text string }{
	{"ada", "the analytical engine finished its run"},
	{"grace", "found the moth; it was in relay 70"},
	{"alan", "the halting question is settled for this input, at least"},
	{"barbara", "the maize plants disagree with the schedule"},
	{"katherine", "recomputed the trajectory, we are fine"},
}

func (r *room) chatter() {
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()
	index := 0
	for {
		select {
		case <-r.stop:
			return
		case now := <-ticker.C:
			line := chatterLines[index%len(chatterLines)]
			index++
			r.post(Message{Author: line.author, Text: line.text, At: now.Format("15:04:05")})
		}
	}
}
