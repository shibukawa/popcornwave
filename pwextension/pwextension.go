// Package pwextension is what a framework plugin needs from the net/http
// runtime without linking it.
//
// It exists because of one asymmetry. A plugin — an authentication provider, a
// storage integration — is transport-free almost everywhere: it reads settings,
// opens stores, and decides things. Two lines of it are not, and they were
// enough to put the whole net/http runtime into every build that imported it,
// including a build serving on the other transport that would never call
// either. Those two lines are here instead.
//
// # What is here
//
// The registration, so a plugin can declare a positioned frame of the net/http
// chain, and the two responses a plugin writes over net/http. Both name
// net/http, which is not the same as naming a runtime: net/http is a protocol
// library, and the thing worth not linking is the framework built on it.
//
// # Why the second transport has no counterpart
//
// pwfast.Middlewares takes its frames as arguments and reads no registry. That
// is deliberate rather than missing: a chain assembled from arguments cannot
// silently gain a frame because something was imported. So this registry is the
// net/http chain's, and a plugin serving both transports registers here and
// hands the other transport a frame directly.
package pwextension

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/shibukawa/popcornwave/pwruntime"
)

// Middleware is the standard net/http middleware shape used by framework
// extensions.
type Middleware = func(http.Handler) http.Handler

// Slot orders every frame of the request chain. The numbers are the shared
// leaf's, so both runtimes compose in one order.
type Slot = pwruntime.Slot

// Extension is one imported framework capability. Setup runs once during
// framework initialization, after configuration parsing and database startup,
// and returns the middleware to install. Returning a nil middleware installs
// nothing, which is how a disabled extension opts out.
type Extension struct {
	Name  string
	Slot  Slot
	Setup func(context.Context) (Middleware, error)
	// Close releases resources owned by the extension during shutdown.
	Close func(context.Context) error
}

var state = struct {
	sync.Mutex
	registered []Extension
}{}

// Register adds one extension to the net/http chain. Imported packages call it
// from an init function so that only linked capabilities contribute
// configuration and code.
func Register(extension Extension) {
	if strings.TrimSpace(extension.Name) == "" {
		panic("popcornwave: empty extension name")
	}
	if extension.Setup == nil {
		panic("popcornwave: extension " + extension.Name + " has no setup")
	}
	state.Lock()
	defer state.Unlock()
	for _, existing := range state.registered {
		if existing.Name == extension.Name {
			panic("popcornwave: duplicate extension " + extension.Name)
		}
	}
	state.registered = append(state.registered, extension)
}

// Registered returns every extension in ascending slot order, so a runtime
// setting them up can rely on an earlier slot having run.
func Registered() []Extension {
	state.Lock()
	ordered := append([]Extension(nil), state.registered...)
	state.Unlock()
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Slot < ordered[j].Slot })
	return ordered
}

// Responders are how this process answers over net/http.
//
// They are published rather than reached for, because rendering a problem is
// the runtime's job: it negotiates a representation, reaches the registered
// error page, and knows whether the response already committed. A plugin needs
// the answer without any of that.
type Responders struct {
	Problem  func(http.ResponseWriter, *http.Request, error)
	Redirect func(http.ResponseWriter, *http.Request, string, int)
}

var responders struct {
	sync.RWMutex
	installed Responders
}

// PublishResponders records how this process answers. The net/http runtime
// calls it once, from an init.
func PublishResponders(installed Responders) {
	responders.Lock()
	responders.installed = installed
	responders.Unlock()
}

// Problem answers with the framework's problem document.
//
// With no runtime linked it writes the document itself, which is the same
// bytes the runtime's API branch writes because both build it from the shared
// leaf. What is lost is the HTML branch — a build with no runtime has no
// registered error page to render — and losing it is the honest outcome rather
// than a failure: the status, the headers and the document are all still
// there, and only the presentation a browser would have preferred is not.
func Problem(w http.ResponseWriter, r *http.Request, err error) {
	responders.RLock()
	write := responders.installed.Problem
	responders.RUnlock()
	if write != nil {
		write(w, r, err)
		return
	}
	writeProblemJSON(w, err)
}

// Redirect sends the browser to another location.
//
// With no runtime linked it falls back to net/http's own helper, which is the
// same resolution the runtime's does. What is lost is the update branch: an
// update request is a fetch, so a runtime answers it with a navigate directive
// rather than a 303. A build with no runtime serves no update, so there is
// nothing there to get wrong.
func Redirect(w http.ResponseWriter, r *http.Request, url string, status int) {
	responders.RLock()
	send := responders.installed.Redirect
	responders.RUnlock()
	if send != nil {
		send(w, r, url, status)
		return
	}
	http.Redirect(w, r, url, status)
}

func writeProblemJSON(w http.ResponseWriter, err error) {
	problem := pwruntime.SanitizeProblem(asProblem(err))
	if applyErr := pwruntime.ApplyProblemHeaders(w.Header(), problem); applyErr != nil {
		// The rate limit metadata was inconsistent. The problem is still the
		// answer, so it is written without the headers rather than swallowed.
		problem.RateLimit = nil
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(problem.Status)
	_, _ = w.Write(pwruntime.AppendProblemJSON(nil, problem))
}

func asProblem(err error) pwruntime.Problem {
	if err == nil {
		return pwruntime.InternalServerError(fmt.Errorf("nil error"))
	}
	var problem pwruntime.Problem
	if errors.As(err, &problem) {
		if problem.Status == 0 {
			problem.Status = http.StatusInternalServerError
		}
		if problem.Title == "" {
			problem.Title = http.StatusText(problem.Status)
		}
		return problem
	}
	return pwruntime.InternalServerError(err)
}
