package authfast

import (
	"bufio"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/plugin/auth"
	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinygodriver/fasthttp"
	"github.com/shibukawa/tinygodriver/fasthttp/fasthttputil"
)

// This package contributes one thing: a reader of a fasthttp request value that
// answers auth.Exchange. Everything it feeds is covered where the decisions
// live, so what is worth proving here is that each accessor reads the field it
// names — and that it reads it off a request the server parsed rather than one
// a test assembled, because half of what could go wrong is decided by the
// parser and the pool.

// serve runs one request through a real fasthttp server and hands the exchange
// to inspect, so the request value under test is a pooled one the transport
// filled in.
func serve(t *testing.T, request string, inspect func(auth.Exchange)) (int, string, string) {
	t.Helper()
	listener := fasthttputil.NewInmemoryListener()
	server := &fasthttp.Server{Handler: func(r *fasthttp.RequestCtx) { inspect(Exchange(r)) }}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("the fasthttp server did not shut down")
		}
	})

	conn, err := listener.Dial()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.Write([]byte(request)); err != nil {
		t.Fatal(err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(conn)
	if err != nil && err != io.EOF && !strings.Contains(err.Error(), "closed") {
		t.Fatal(err)
	}
	var response fasthttp.Response
	if err := response.Read(bufio.NewReader(strings.NewReader(string(raw)))); err != nil {
		t.Fatalf("unreadable response: %v\n%s", err, raw)
	}
	return response.StatusCode(), string(response.Header.Header()), string(response.Body())
}

func get(target, extraHeaders string) string {
	return "GET " + target + " HTTP/1.1\r\nHost: app.example\r\n" + extraHeaders + "Connection: close\r\n\r\n"
}

func TestTheExchangeReadsTheRequestLine(t *testing.T) {
	var got []string
	serve(t, get("/admin/users?page=2&flag", ""), func(x auth.Exchange) {
		got = []string{x.Method(), x.Path(), x.Target(), x.Query("page"), x.Query("missing"), x.Host()}
	})

	want := []string{"GET", "/admin/users", "/admin/users?page=2&flag", "2", "", "app.example"}
	for index, value := range want {
		if got[index] != value {
			t.Errorf("field %d = %q, want %q", index, got[index], value)
		}
	}
}

// RawPath is the one accessor whose whole point is what it does not do: it must
// keep the encoded separator the decoded path no longer has, and it must not
// carry the query string, because a %2F in a parameter is not an ambiguous path.
func TestRawPathIsThePathBeforeDecoding(t *testing.T) {
	var encoded, withQuery string
	serve(t, get("/a%2Fb", ""), func(x auth.Exchange) { encoded = x.RawPath() })
	serve(t, get("/admin?next=%2Fdashboard", ""), func(x auth.Exchange) { withQuery = x.RawPath() })

	if encoded != "/a%2Fb" {
		t.Errorf("RawPath = %q, want the undecoded path", encoded)
	}
	if withQuery != "/admin" {
		t.Errorf("RawPath = %q, want the path alone", withQuery)
	}
}

func TestTheExchangeReadsHeadersAndCookies(t *testing.T) {
	var single, first string
	var all []string
	var cookies []*http.Cookie
	serve(t, get("/", "Origin: https://app.example\r\nAuthorization: Bearer one\r\nAuthorization: Bearer two\r\nCookie: a=1; b=2\r\n"),
		func(x auth.Exchange) {
			single = x.Header("origin")
			all = x.HeaderValues("Authorization")
			first = x.Header("Authorization")
			cookies = x.Cookies()
		})

	if single != "https://app.example" {
		t.Errorf("Header = %q; a header name is matched without regard to case", single)
	}
	// Two Authorization headers are refused rather than merged by the verifier,
	// and it can only refuse what it can see.
	if len(all) != 2 {
		t.Fatalf("HeaderValues returned %d values, want both: %q", len(all), all)
	}
	if first != all[0] {
		t.Errorf("Header returned %q, want the first of %q", first, all)
	}
	if len(cookies) != 2 || cookies[0].Name != "a" || cookies[1].Value != "2" {
		t.Fatalf("cookies = %+v", cookies)
	}
}

// A form field comes from the submitted body and never from the query. Reading
// the query too would make a logout scope settable by a link, which is exactly
// what a POST-only endpoint exists to prevent.
func TestFormValueReadsTheBodyOnly(t *testing.T) {
	var fromBody, fromQuery string
	body := "scope=global"
	serve(t, "POST /auth/logout?scope=global HTTP/1.1\r\nHost: app.example\r\n"+
		"Content-Type: application/x-www-form-urlencoded\r\nContent-Length: "+strconv.Itoa(len(body))+
		"\r\nConnection: close\r\n\r\n"+body,
		func(x auth.Exchange) { fromBody = x.FormValue("scope") })
	serve(t, "POST /auth/logout?scope=global HTTP/1.1\r\nHost: app.example\r\n"+
		"Content-Length: 0\r\nConnection: close\r\n\r\n",
		func(x auth.Exchange) { fromQuery = x.FormValue("scope") })

	if fromBody != "global" {
		t.Errorf("FormValue from the body = %q, want global", fromBody)
	}
	if fromQuery != "" {
		t.Errorf("FormValue read the query: %q", fromQuery)
	}
}

// An oversized body is refused as one rather than truncated, so an endpoint
// answers "too large" instead of "malformed".
func TestBodyRefusesWhatIsOverTheLimit(t *testing.T) {
	body := strings.Repeat("x", 64)
	request := "POST / HTTP/1.1\r\nHost: app.example\r\nContent-Length: " + strconv.Itoa(len(body)) +
		"\r\nConnection: close\r\n\r\n" + body

	var within []byte
	var withinErr, overErr error
	serve(t, request, func(x auth.Exchange) { within, withinErr = x.Body(128) })
	serve(t, request, func(x auth.Exchange) { _, overErr = x.Body(16) })

	if withinErr != nil {
		t.Fatalf("a body inside the limit was refused: %v", withinErr)
	}
	if string(within) != body {
		t.Errorf("Body returned %d bytes, want %d", len(within), len(body))
	}
	if overErr == nil {
		t.Error("a body over the limit was accepted")
	}
}

// The response side commits what it was given, and the header survives
// serialization — which on this transport is the part a recorder would not
// have proved.
func TestTheExchangeWritesTheResponse(t *testing.T) {
	status, header, body := serve(t, get("/", ""), func(x auth.Exchange) {
		x.SetHeader("Cache-Control", "no-store")
		x.SetHeader("Content-Type", "application/json")
		x.Write(http.StatusTeapot, []byte(`{"ok":true}`))
	})

	if status != http.StatusTeapot {
		t.Errorf("status = %d", status)
	}
	if body != `{"ok":true}` {
		t.Errorf("body = %q", body)
	}
	for _, fragment := range []string{"Cache-Control: no-store", "Content-Type: application/json"} {
		if !strings.Contains(header, fragment) {
			t.Errorf("header does not carry %q:\n%s", fragment, header)
		}
	}
}

// A cookie set through the exchange arrives with the attributes it was given.
// The translation is the session package's, reached through the shared carrier,
// and this is the one place it is exercised for a login cookie.
func TestACookieSetThroughTheExchangeCarriesItsAttributes(t *testing.T) {
	_, header, _ := serve(t, get("/", ""), func(x auth.Exchange) {
		x.SetCookie(&http.Cookie{
			Name: "pw_session_txn", Value: "opaque", Path: "/auth/callback",
			MaxAge: 600, HttpOnly: true, SameSite: http.SameSiteLaxMode,
		})
		x.Write(http.StatusNoContent, nil)
	})

	for _, fragment := range []string{"pw_session_txn=opaque", "path=/auth/callback", "max-age=600", "HttpOnly", "SameSite=Lax"} {
		if !strings.Contains(header, fragment) {
			t.Errorf("Set-Cookie does not carry %q:\n%s", fragment, header)
		}
	}
}

// A recorded authentication is readable by everything downstream, which on this
// transport means a write into the pooled request value rather than a derived
// context. A frame that recorded into a context nobody kept would leave every
// signed-in visitor looking anonymous to the guard.
func TestARecordedAuthenticationIsReadableDownstream(t *testing.T) {
	_, _, body := serve(t, get("/", ""), func(x auth.Exchange) {
		if pwruntime.RequestAuthentication(x.Context()).Authenticated {
			x.Write(http.StatusOK, []byte("already authenticated"))
			return
		}
		x.RecordAuthentication(pwruntime.Authentication{
			Authenticated: true, Subject: "account-7", Method: auth.MethodOIDC,
		})
		read := pwruntime.RequestAuthentication(x.Context())
		x.Write(http.StatusOK, []byte(read.Method+":"+read.Subject))
	})

	if body != "oidc:account-7" {
		t.Errorf("the recorded authentication read back as %q", body)
	}
}

// The peer is the parsed address rather than whatever net.Addr the listener
// produced, because an in-memory listener's is a name and an unparseable peer
// is never trusted by anything that reads it.
func TestRemoteAddressIsAParsedAddress(t *testing.T) {
	var address string
	serve(t, get("/", ""), func(x auth.Exchange) { address = x.RemoteAddress() })

	if address == "" {
		t.Fatal("no remote address was reported")
	}
	if strings.ContainsAny(address, "/@") {
		t.Errorf("RemoteAddress = %q, which is a name rather than an address", address)
	}
}
