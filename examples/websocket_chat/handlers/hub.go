package handlers

import "sync"

// The framework holds one connection: its limits, its deadlines, its close
// handshake. It holds no registry of connections, because who is in the room is
// the application's question. This is that registry, and it is the whole of what
// a chat room adds.
//
// It names no transport package. A member is anything that can be written to,
// and the typed socket both builds hand the callback satisfies it — so this file
// compiles unchanged for net/http and for fasthttp, with nothing to rewrite.
type member interface {
	Write(ServerMsg) error
}

type room struct {
	mu      sync.Mutex
	members map[member]string
}

var chat = &room{members: map[member]string{}}

func (r *room) join(who member, name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.members[who] = name
}

func (r *room) leave(who member) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name, joined := r.members[who]
	delete(r.members, who)
	return name, joined
}

func (r *room) name(who member) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name, joined := r.members[who]
	return name, joined
}

func (r *room) size() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.members)
}

// broadcast writes to every member from the caller's goroutine, which is the
// payoff of the socket serializing writes: one goroutine may write to a hundred
// connections without a frame from one message interleaving with another.
//
// A failed write is dropped rather than reported. The peer it belonged to is
// already on its way out, and its own read loop is what notices.
func (r *room) broadcast(message ServerMsg) {
	r.mu.Lock()
	targets := make([]member, 0, len(r.members))
	for who := range r.members {
		targets = append(targets, who)
	}
	r.mu.Unlock()

	for _, who := range targets {
		_ = who.Write(message)
	}
}
