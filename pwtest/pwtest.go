// Package pwtest is the backend-neutral vocabulary a test uses to describe one
// request and inspect one response.
//
// It exists because of a counting exercise. This repository drives handlers
// through httptest in 86 test files, 66 of them by building a request, calling
// a handler, and reading a recorder. Every one of those is written against
// net/http's request and response types, so porting the framework to a second
// transport without a seam means writing all 66 again — and a test written
// twice is two tests that can disagree about what the framework does, which is
// the opposite of what a test is for.
//
// So the request and the response are described here, in types that name no
// transport, and each transport supplies one Exchange that runs the description
// through a real server of its own kind. A test written against this pair says
// the same thing on both, and the only line that differs is the import.
//
// # Why a real server rather than a recorder
//
// A recorder is cheaper and answers a different question. Half of what a
// framework entry does is decided by the transport — whether the response
// commits, whether a header survives serialization, what a pooled request value
// carries — and a hand-built request value tests the entry against the test's
// idea of the transport rather than against the transport. Both Exchange
// implementations run a real server over an in-memory pipe, which costs no
// socket and keeps the answer honest.
//
// # Nothing here imports a transport
//
// Not net/http, not the fasthttp fork, not testing. That is the whole point:
// this package is what the two sides agree on, so it cannot be allowed to
// prefer one of them.
package pwtest

import (
	"sort"
	"strings"
)

// TestingT is the minimal testing surface, the same one testutil accepts and
// for the same reason: a shipped package must not import testing.
//
// It is declared here rather than in either transport's helper so both accept
// the same interface and a caller's own T wrapper satisfies both at once.
type TestingT interface {
	Helper()
	Cleanup(func())
	Fatalf(string, ...any)
	Errorf(string, ...any)
}

// Request describes one request to make.
//
// The zero value is a GET of "/" with no headers and no body, because that is
// the request most tests want and spelling it out every time would bury the
// part that differs.
type Request struct {
	// Method defaults to GET.
	Method string
	// Target is the request target: a path with an optional query. It defaults
	// to "/". It is not a URL, because the host is the server's rather than the
	// test's.
	Target string
	// Header carries the request headers. Names are matched case-insensitively
	// on the way out, as HTTP requires.
	Header map[string][]string
	// Body is sent as-is. A test setting one usually sets Content-Type too;
	// nothing here guesses it, because a wrong guess would be a header the
	// handler branches on that the test did not write.
	Body []byte
}

// Method, Target and headers are read through accessors so both Exchange
// implementations apply the defaults identically rather than each remembering
// to.

// ResolvedMethod is the method to send.
func (r Request) ResolvedMethod() string {
	if r.Method == "" {
		return "GET"
	}
	return r.Method
}

// ResolvedTarget is the target to send.
func (r Request) ResolvedTarget() string {
	if r.Target == "" {
		return "/"
	}
	return r.Target
}

// SortedHeader returns the headers in a stable order, so a request built from a
// map produces the same bytes on both transports and a failure is reproducible.
func (r Request) SortedHeader() []Header {
	var headers []Header
	for name, values := range r.Header {
		for _, value := range values {
			headers = append(headers, Header{Name: name, Value: value})
		}
	}
	sort.Slice(headers, func(i, j int) bool {
		if headers[i].Name != headers[j].Name {
			return headers[i].Name < headers[j].Name
		}
		return headers[i].Value < headers[j].Value
	})
	return headers
}

// Header is one name and value.
type Header struct{ Name, Value string }

// Response is what a handler answered.
type Response struct {
	Status int
	Header map[string][]string
	Body   []byte
}

// Text is the body as a string, which is what most assertions compare.
func (r Response) Text() string { return string(r.Body) }

// Get returns the first value of a response header, matched
// case-insensitively, or the empty string.
//
// Case-insensitive because the two transports canonicalise header names
// differently — one gives X-Request-Id and the other X-Request-ID — and a test
// asserting on a header should not have to know which server answered it.
func (r Response) Get(name string) string {
	for key, values := range r.Header {
		if strings.EqualFold(key, name) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

// Has reports whether a response header is present at all, which is the
// assertion for a header whose value is not the point.
func (r Response) Has(name string) bool {
	for key, values := range r.Header {
		if strings.EqualFold(key, name) && len(values) > 0 {
			return true
		}
	}
	return false
}

// Values returns every value of a response header, for the few headers that
// legitimately repeat — Set-Cookie above all.
func (r Response) Values(name string) []string {
	var found []string
	for key, values := range r.Header {
		if strings.EqualFold(key, name) {
			found = append(found, values...)
		}
	}
	return found
}
