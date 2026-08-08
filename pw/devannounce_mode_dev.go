//go:build pwdev

package pw

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// DevAttachTokenVar carries the per-run token pw dev generates. The console
// accepts an announcement only when it matches, so an attachment is not taken
// from anything that merely reached the port.
const DevAttachTokenVar = "PW_DEV_ATTACH_TOKEN"

// announceDevelopmentListener tells the console where this process is actually
// listening.
//
// The console links the application from its index, and the only address it can
// work out on its own comes from reading the development configuration file.
// That is a guess: an environment variable or a flag outranks the file, and a
// development run moves off a port it cannot bind. The process that owns the
// listener is the one place the answer is not a guess, so it says so.
func announceDevelopmentListener(url string) {
	console := strings.TrimSpace(os.Getenv(DevConsoleURLVar))
	if console == "" || url == "" {
		// No console is running, so there is nothing to announce to. This is the
		// ordinary case for a pwdev binary started by hand.
		return
	}
	announceToDevConsole(console, "/api/listening", url, "listening address")
}

// announceToDevConsole delivers one announcement.
//
// The application dials out; the console never dials in. That is what keeps a
// development surface off the application's own listener while still letting one
// page reach it, and it is the direction the telemetry exporter already uses.
//
// The client is bounded because these calls sit between configuration and the
// first accepted request: an unanswered announcement should cost a line on
// stderr, not the application.
func announceToDevConsole(console, path, body, what string) {
	request, err := http.NewRequest(http.MethodPost, console+path, strings.NewReader(body))
	if err != nil {
		return
	}
	request.Header.Set("Content-Type", "text/plain")
	request.Header.Set("X-Pw-Attach-Token", os.Getenv(DevAttachTokenVar))
	response, err := (&http.Client{Timeout: 3 * time.Second}).Do(request)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pw: %s: could not announce to the console: %v\n", what, err)
		return
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		// A refused announcement means the console will never learn this, and it
		// can only report what it guessed or that it is waiting. Saying why here
		// is the only place the reason exists.
		fmt.Fprintf(os.Stderr, "pw: %s: the console refused the announcement (%s)\n", what, response.Status)
	}
}
