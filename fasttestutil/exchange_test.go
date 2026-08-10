package fasttestutil_test

import (
	"net/http"
	"testing"

	"github.com/shibukawa/popcornwave/fasttestutil"
	"github.com/shibukawa/popcornwave/pwtest"
	"github.com/shibukawa/popcornwave/testutil"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// The seam is only worth having if the two halves agree, so the assertion is
// the agreement rather than each half's own output.
//
// Both handlers are written to do the same four things — read a header, read
// the query, set a header, answer with a status and a body — and the two
// responses are then compared field by field. A difference here is a difference
// in the framework's transports, which is exactly what a test written once and
// run on both is for.
func TestBothHalvesAnswerTheSameDescriptionTheSameWay(t *testing.T) {
	request := pwtest.Request{
		Method: "POST",
		Target: "/orders?page=2",
		Header: map[string][]string{
			"X-Caller":     {"conformance"},
			"Content-Type": {"text/plain"},
		},
		Body: []byte("payload"),
	}

	overNetHTTP := testutil.Exchange(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Seen-Caller", r.Header.Get("X-Caller"))
		w.Header().Set("X-Seen-Page", r.URL.Query().Get("page"))
		w.WriteHeader(http.StatusAccepted)
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		_, _ = w.Write(body)
	}), request)

	overFastHTTP := fasttestutil.Exchange(t, func(r *fasthttp.RequestCtx) {
		r.Response.Header.Set("X-Seen-Caller", string(r.Request.Header.Peek("X-Caller")))
		r.Response.Header.Set("X-Seen-Page", string(r.QueryArgs().Peek("page")))
		r.SetStatusCode(fasthttp.StatusAccepted)
		_, _ = r.Write(r.Request.Body())
	}, request)

	if overNetHTTP.Status != overFastHTTP.Status {
		t.Errorf("status: net/http %d, fasthttp %d", overNetHTTP.Status, overFastHTTP.Status)
	}
	if overNetHTTP.Text() != overFastHTTP.Text() {
		t.Errorf("body: net/http %q, fasthttp %q", overNetHTTP.Text(), overFastHTTP.Text())
	}
	// Read through Get, which matches case-insensitively: the two servers
	// canonicalise header names differently and a test should not have to know
	// which one answered.
	for _, name := range []string{"X-Seen-Caller", "X-Seen-Page"} {
		if overNetHTTP.Get(name) != overFastHTTP.Get(name) {
			t.Errorf("%s: net/http %q, fasthttp %q", name,
				overNetHTTP.Get(name), overFastHTTP.Get(name))
		}
	}
	if overNetHTTP.Get("X-Seen-Caller") != "conformance" {
		t.Errorf("the request header did not reach either handler: %q", overNetHTTP.Get("X-Seen-Caller"))
	}
	if overNetHTTP.Text() != "payload" {
		t.Errorf("the request body did not reach either handler: %q", overNetHTTP.Text())
	}
}

// The zero Request is the one most tests want, so it has to mean something.
func TestTheZeroRequestIsAGetOfTheRoot(t *testing.T) {
	seen := ""
	response := fasttestutil.Exchange(t, func(r *fasthttp.RequestCtx) {
		seen = string(r.Method()) + " " + string(r.Path())
	}, pwtest.Request{})

	if seen != "GET /" {
		t.Errorf("the zero request arrived as %q", seen)
	}
	if response.Status != fasthttp.StatusOK {
		t.Errorf("status = %d", response.Status)
	}
}

// A repeating header is the case Get cannot answer, and Set-Cookie is why the
// accessor for it exists.
func TestValuesReturnsEveryHeaderOfAName(t *testing.T) {
	response := fasttestutil.Exchange(t, func(r *fasthttp.RequestCtx) {
		r.Response.Header.Add("Set-Cookie", "a=1")
		r.Response.Header.Add("Set-Cookie", "b=2")
	}, pwtest.Request{})

	if got := response.Values("set-cookie"); len(got) != 2 {
		t.Errorf("Values returned %v, want two cookies", got)
	}
}
