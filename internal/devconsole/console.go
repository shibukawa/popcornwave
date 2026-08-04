package devconsole

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Pane is one surface on the console. A pane is a page rather than a region of
// a shared document: an embedded third-party renderer ships CSS written to own
// a document, and giving it one costs less than isolating it inside a shell.
type Pane struct {
	// Slug is the first path segment the pane is mounted under, and the id the
	// index links to.
	Slug    string
	Title   string
	Summary string
	// Handler serves the pane below its slug, with the slug already stripped.
	// A nil handler means the pane is disabled.
	Handler http.Handler
	// DisabledBy names the configuration key that would enable a disabled
	// pane. The index says this rather than hiding the pane, so a developer
	// who expected a surface learns why it is not there.
	DisabledBy string
	// RootPaths are absolute console paths this pane owns outside its own
	// subtree.
	//
	// This exists for one reason. The telemetry UI is a committed build whose
	// bundle resolves its API against the document origin rather than against
	// its own base, so mounting the page under a prefix does not move its
	// fetches with it. Teaching it otherwise means rebuilding the bundle,
	// which needs the Node toolchain the embedded build exists to avoid.
	RootPaths []string
}

// Enabled reports whether the pane has a handler. It is exported because the
// index template asks each pane whether to link it or explain it.
func (p Pane) Enabled() bool { return p.Handler != nil }

// Project is what the index says about the project itself.
type Project struct {
	Name string
	// Environment is the APP_ENV the loop runs the application under.
	Environment string
	// ApplicationURL is where the application listens, or empty when pw could
	// not determine it. The index reports an undetermined value as
	// undetermined rather than printing a default that may be wrong.
	ApplicationURL string
	// APIDocURL is the documentation UI the application already serves, at the
	// path its configuration puts it. The console links it rather than
	// rendering the specification itself: a second renderer would be a second
	// thing to keep current with the same document, and a link cannot disagree
	// with what the application serves.
	//
	// Empty means the endpoint is off, or that its path could not be resolved.
	APIDocURL string
	// APIDocKey names the configuration key that turns the endpoint on, for the
	// index to quote when there is no URL to link.
	APIDocKey string
}

// Console is the listener, the panes mounted on it, and the current loop state.
type Console struct {
	listener net.Listener
	server   *http.Server
	state    *stateHolder
	project  Project
	panes    []Pane
}

// New binds the console listener and serves the index and every pane on it.
//
// The address is fixed by configuration rather than reserved, so a bound port
// is a real failure with a real remedy rather than something to route around.
func New(address string, project Project, panes []Pane) (*Console, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", address, err)
	}
	sort.SliceStable(panes, func(i, j int) bool { return panes[i].Slug < panes[j].Slug })
	console := &Console{
		listener: listener,
		state:    &stateHolder{},
		project:  project,
		panes:    panes,
	}
	console.server = &http.Server{Handler: console.routes()}
	go console.server.Serve(listener)
	return console, nil
}

func (c *Console) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", c.index)
	mux.HandleFunc("GET /api/loop-state", c.loopState)
	mux.HandleFunc("GET /api/loop-state/stream", c.loopStateStream)
	for _, pane := range c.panes {
		if !pane.Enabled() {
			continue
		}
		prefix := "/" + pane.Slug
		mux.Handle(prefix+"/", http.StripPrefix(prefix, c.withNav(pane.Handler)))
		// A pane reached without its trailing slash would resolve its own
		// relative asset references against the console root, so the redirect
		// is what keeps a hand-typed URL working.
		mux.Handle("GET "+prefix, http.RedirectHandler(prefix+"/", http.StatusMovedPermanently))
		for _, path := range pane.RootPaths {
			mux.Handle(path, pane.Handler)
		}
	}
	return mux
}

// withNav hands a pane the console navigation. A pane that renders with the
// console layout needs the same nav as every other page, and passing it through
// the request keeps a pane constructor from taking a Console that does not exist
// yet when the pane is built.
func (c *Console) withNav(handler http.Handler) http.Handler {
	nav := c.nav()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), navKey{}, nav)))
	})
}

func (c *Console) nav() []navPane {
	nav := make([]navPane, 0, len(c.panes))
	for _, pane := range c.panes {
		nav = append(nav, navPane{Slug: pane.Slug, Title: pane.Title, Enabled: pane.Enabled()})
	}
	return nav
}

// URL is the one address api:cli-dev prints for the console.
func (c *Console) URL() string {
	if c == nil {
		return ""
	}
	return "http://" + c.listener.Addr().String()
}

// Publish records a loop transition. It is safe on a nil console, because a
// console that failed to listen must not turn every phase change into a branch
// at the call site.
func (c *Console) Publish(phase string, status Status, diagnostic *Diagnostic) {
	if c == nil {
		return
	}
	c.state.publish(phase, status, diagnostic, time.Now())
}

// Failed is the common case of Publish: a phase that produced a diagnostic.
func (c *Console) Failed(phase string, text string) {
	if c == nil || strings.TrimSpace(text) == "" {
		return
	}
	c.Publish(phase, StatusFailed, &Diagnostic{Text: text})
}

// State reports what the console currently holds, for callers that render it
// themselves.
func (c *Console) State() State {
	if c == nil {
		return State{}
	}
	return c.state.get()
}

func (c *Console) loopState(w http.ResponseWriter, r *http.Request) {
	allowLoopbackOrigin(w, r)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(c.state.get())
}

// loopStateStream pushes the current state and then every transition.
//
// The stream terminates here rather than at the application, which is the whole
// point of it: a page keeps being told what the loop is doing while the process
// that served it is stopped, which is exactly when a developer wants to know.
func (c *Console) loopStateStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	allowLoopbackOrigin(w, r)
	header := w.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-store")
	// The console is reached directly on loopback, but a proxy between them
	// would otherwise buffer the stream into uselessness.
	header.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	woken, stop := c.state.watch()
	defer stop()
	// The first record goes out before anything is awaited, so a page that
	// connects after the transition it cares about is still told about it.
	if !writeStateEvent(w, flusher, c.state.get()) {
		return
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case <-woken:
			if !writeStateEvent(w, flusher, c.state.get()) {
				return
			}
		}
	}
}

func writeStateEvent(w http.ResponseWriter, flusher http.Flusher, state State) bool {
	encoded, err := json.Marshal(state)
	if err != nil {
		return false
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", encoded); err != nil {
		return false
	}
	flusher.Flush()
	return true
}

// allowLoopbackOrigin lets a page served by the application read this response.
//
// The application and the console are two ports, so a page from one is a
// different origin to the other and a plain fetch would be refused. The
// allowance is echoed per request and only for a loopback origin: the console
// binds to loopback anyway, so this widens nothing, and naming the origin back
// rather than answering "*" keeps the response uncacheable across origins.
func allowLoopbackOrigin(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" || !loopbackOrigin(origin) {
		return
	}
	header := w.Header()
	header.Set("Access-Control-Allow-Origin", origin)
	header.Add("Vary", "Origin")
}

func loopbackOrigin(origin string) bool {
	address, ok := strings.CutPrefix(origin, "http://")
	if !ok {
		return false
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	// An IPv6 literal arrives bracketed, and SplitHostPort has already
	// unwrapped it when a port was present.
	return net.ParseIP(strings.Trim(host, "[]")).IsLoopback()
}

// Close stops the console with the developer loop. Nothing here is persisted,
// so there is nothing to flush.
func (c *Console) Close() {
	if c == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = c.server.Shutdown(ctx)
}
