package lobby

import "sync"

// Hub is the in-process fan-out seam for lobby changes. The database remains
// the source of truth; replacing this package with pub/sub does not change the
// page source or its rendering contract.
type Hub struct {
	mu        sync.Mutex
	listeners map[chan struct{}]struct{}
}

var rooms = &Hub{listeners: make(map[chan struct{}]struct{})}

var roomChanges = struct {
	sync.Mutex
	listeners map[int]map[chan struct{}]struct{}
}{listeners: make(map[int]map[chan struct{}]struct{})}

// Subscribe returns a coalescing change signal and an unsubscribe function.
func Subscribe() (<-chan struct{}, func()) {
	listener := make(chan struct{}, 1)
	rooms.mu.Lock()
	rooms.listeners[listener] = struct{}{}
	rooms.mu.Unlock()

	return listener, func() {
		rooms.mu.Lock()
		delete(rooms.listeners, listener)
		rooms.mu.Unlock()
	}
}

// NotifyRoomsChanged wakes every current lobby viewer without blocking the
// mutation that produced the change.
func NotifyRoomsChanged() {
	rooms.mu.Lock()
	defer rooms.mu.Unlock()
	for listener := range rooms.listeners {
		select {
		case listener <- struct{}{}:
		default:
		}
	}
}

// SubscribeRoom returns a coalescing change signal for one room.
func SubscribeRoom(roomID int) (<-chan struct{}, func()) {
	listener := make(chan struct{}, 1)
	roomChanges.Lock()
	listeners := roomChanges.listeners[roomID]
	if listeners == nil {
		listeners = make(map[chan struct{}]struct{})
		roomChanges.listeners[roomID] = listeners
	}
	listeners[listener] = struct{}{}
	roomChanges.Unlock()

	return listener, func() {
		roomChanges.Lock()
		listeners := roomChanges.listeners[roomID]
		delete(listeners, listener)
		if len(listeners) == 0 {
			delete(roomChanges.listeners, roomID)
		}
		roomChanges.Unlock()
	}
}

// NotifyRoomChanged wakes every viewer of the changed room without blocking
// the mutation that produced it.
func NotifyRoomChanged(roomID int) {
	roomChanges.Lock()
	defer roomChanges.Unlock()
	for listener := range roomChanges.listeners[roomID] {
		select {
		case listener <- struct{}{}:
		default:
		}
	}
}
