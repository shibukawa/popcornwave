package pw

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

// HTMLErrorPage resolves the fragment shown in place of a page whose rendering
// failed. It receives the mapped problem rather than the original error, so a
// template can never render a cause the server meant to keep.
type HTMLErrorPage func(Problem) HTMLFragment

var errorPageState = struct {
	sync.RWMutex
	resolve HTMLErrorPage
}{}

// RegisterHTMLErrorPage installs the application's error page resolver. It is
// intended for generated templates code and for an application that wants its
// own presentation; without one, a minimal built-in page is used.
func RegisterHTMLErrorPage(resolve HTMLErrorPage) {
	errorPageState.Lock()
	defer errorPageState.Unlock()
	errorPageState.resolve = resolve
}

func registeredHTMLErrorPage() HTMLErrorPage {
	errorPageState.RLock()
	defer errorPageState.RUnlock()
	return errorPageState.resolve
}

// writeDocumentEscalation replaces everything below the document shell with an
// error page.
//
// It exists because a boundary that failed with no recover clause has nothing
// to put in its own placeholder: the template said what to show while waiting
// and what to show on success, and nothing about failure. Leaving the committed
// fallback in place would make the page claim forever that it is still loading.
//
// The status went out with the shell, so this changes only what a reader sees.
// The failure reaches an operator through Logger, never through the status line.
func writeDocumentEscalation(w io.Writer, problem Problem) error {
	var body bytes.Buffer
	if resolve := registeredHTMLErrorPage(); resolve != nil {
		fragment := resolve(problem)
		if fragment.Present() {
			if err := htmlbind.Render(&body, fragment); err != nil {
				// The error page is the last thing standing between a reader and
				// a permanent loading state, so its own failure falls back to the
				// built-in rather than propagating.
				body.Reset()
				builtinErrorPage(&body, problem)
			}
		} else {
			builtinErrorPage(&body, problem)
		}
	} else {
		builtinErrorPage(&body, problem)
	}
	// Same framing discipline as a boundary completion: an inert template, then
	// a marker that commits it. A parser inserts an element at its start tag, so
	// a runtime reacting to the template could read one whose content had not
	// arrived. The marker cannot exist before its template is closed.
	if _, err := io.WriteString(w, `<template data-tb-document>`); err != nil {
		return err
	}
	if _, err := body.WriteTo(w); err != nil {
		return err
	}
	_, err := io.WriteString(w, `</template><tb-apply-document></tb-apply-document>`)
	return err
}

// builtinErrorPage writes the fallback presentation. It carries the status and
// its standard title only: everything else about the failure is server-side.
func builtinErrorPage(w io.Writer, problem Problem) {
	status := problem.Status
	if status == 0 {
		status = 500
	}
	title := problem.Title
	if title == "" {
		title = http.StatusText(status)
	}
	_, _ = io.WriteString(w, `<main><h1>`+strconv.Itoa(status)+` `+htmlbind.Escape(title)+`</h1></main>`)
}

// publicProblem bounds what an error page is allowed to say, by environment
// rather than by status or by client.
//
// In development the reader is the person who caused the failure and is about
// to fix it, so the page carries everything the problem does. Anywhere else the
// same page is served to the public, and it says what went wrong without saying
// why. The template is the same in both: the difference is what it is given,
// because a template that decides this itself decides it once and then gets
// copied into an application that meant something else by it.
func publicProblem(problem Problem) Problem {
	if Development() {
		return problem
	}
	return Problem{Status: problem.Status, Title: problem.Title}
}

// acceptsHTML reports whether the client would rather have a page than a
// document. An absent, empty, or unreadable Accept is not a preference, so it
// takes the API representation, which is also what a client that sent no
// opinion at all is most likely to be.
func acceptsHTML(r *http.Request) bool {
	if r == nil {
		return false
	}
	htmlQuality, jsonQuality := -1.0, -1.0
	for entry := range splitSeq(r.Header.Get("Accept"), ',') {
		media, parameters, _ := strings.Cut(entry, ";")
		media = strings.TrimSpace(strings.ToLower(media))
		quality := 1.0
		for parameter := range splitSeq(parameters, ';') {
			name, value, found := strings.Cut(strings.TrimSpace(parameter), "=")
			if !found || strings.TrimSpace(name) != "q" {
				continue
			}
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
				quality = parsed
			}
		}
		switch media {
		case "text/html", "application/xhtml+xml":
			htmlQuality = max(htmlQuality, quality)
		case "application/json", "application/problem+json":
			jsonQuality = max(jsonQuality, quality)
		}
	}
	return htmlQuality > 0 && htmlQuality >= jsonQuality
}

// writeHTMLProblem answers an uncommitted HTML request with the error page and
// its real status.
//
// It exists so the two render branches tell the same story: the streaming one
// can only patch an error page into a response that already said 200, and this
// one can say 500 while showing the reader the same thing.
//
// It renders through the same wrapper chain the failed page used, so the error
// page keeps the document shell instead of arriving as a bare fragment.
func writeHTMLProblem(w http.ResponseWriter, r *http.Request, wrappers []HTMLWrapper, problem Problem) {
	resolve := registeredHTMLErrorPage()
	if resolve == nil {
		writeProblemJSON(w, r, problem)
		return
	}
	problem = sanitizedProblem(problem)
	fragment := resolve(publicProblem(problem))
	if !fragment.Present() {
		writeProblemJSON(w, r, problem)
		return
	}
	addVaryHeader(w.Header(), "Accept")
	var body bytes.Buffer
	if err := htmlbind.RenderChain(&body, wrappers, fragment); err != nil {
		// Never let an error page's own failure recurse into another one.
		Logger(requestContext(r)).Log(requestContext(r), LevelError, "HTML error page render failed", Err(err))
		writeProblemJSON(w, r, problem)
		return
	}
	status := problem.Status
	if status == 0 {
		status = http.StatusInternalServerError
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(body.Len()))
	w.WriteHeader(status)
	if _, err := body.WriteTo(w); err != nil {
		Logger(requestContext(r)).Log(requestContext(r), LevelError, "HTML error response write failed", Err(err))
	}
}
