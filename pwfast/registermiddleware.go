package pwfast

import (
	"fmt"
	"strings"
	"sync"
)

// applicationMiddleware is what an application registered for this process, in
// the order it registered.
var applicationMiddleware struct {
	sync.Mutex
	frames []Frame
}

// RegisterMiddleware adds one application middleware to the request chain at
// slot. A smaller slot runs earlier, so SlotAccessLog - 5 observes the request
// ID and appears in the access log's timing, and SlotGuard + 1 runs only for
// requests the guard admitted.
//
// It is pw.RegisterMiddleware with one word changed, and that is the point.
// The middleware body is written per transport — decision:backend-specific-middleware
// settles why, and the signature here says it plainly — so the registration is
// the one part of the arrangement that had no reason to differ, and a project
// building both ways should find its two mains differing only in the body.
//
// Call it from main, before Run, Start or Middlewares composes the chain,
// exactly as RegisterSessionStore requires: the chain is composed once, and a
// middleware registered after that joins nothing. A serverless build registers
// in the same place, because main runs as far as its Run call before the
// generated wrapper captures it.
//
// The two fixed frames, SlotOperational and SlotAPIDoc, are handlers rather
// than middleware and refuse registration at their exact number; register one
// position to either side instead. A duplicate name and a nil middleware are
// each a panic at registration rather than a silent gap in the chain — the same
// three refusals the other build makes, so one rule covers both.
//
// This is the application's seam and not a plugin's. An imported capability
// still hands this transport a frame through RuntimeOptions.Extra, where the
// application names it: a plugin registering from an init would be a frame in
// the chain that nothing in the application's source mentions.
func RegisterMiddleware(slot Slot, name string, middleware Middleware) {
	if strings.TrimSpace(name) == "" {
		panic("popcornweb: empty middleware name")
	}
	if middleware == nil {
		panic("popcornweb: middleware " + name + " is nil")
	}
	if slot == SlotOperational || slot == SlotAPIDoc {
		panic(fmt.Sprintf(
			"popcornweb: middleware %s registered at fixed frame %d; pick a neighboring slot relative to pwfast.SlotOperational or pwfast.SlotAPIDoc",
			name, slot))
	}
	applicationMiddleware.Lock()
	defer applicationMiddleware.Unlock()
	for _, existing := range applicationMiddleware.frames {
		if existing.Name == name {
			panic("popcornweb: duplicate middleware " + name)
		}
	}
	applicationMiddleware.frames = append(applicationMiddleware.frames,
		Frame{Slot: slot, Name: name, Middleware: middleware})
}

// registeredMiddleware returns a copy of what was registered, in registration
// order, so equal slots compose in the order main wrote them.
func registeredMiddleware() []Frame {
	applicationMiddleware.Lock()
	defer applicationMiddleware.Unlock()
	return append([]Frame(nil), applicationMiddleware.frames...)
}
