package pwfast

import (
	"strings"
	"testing"

	"github.com/shibukawa/popcornweb/pwruntime"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

func TestRedirectSeeOtherSendsTheLocationAndA303(t *testing.T) {
	status, header, _ := serve(t, func(r *fasthttp.RequestCtx) {
		RedirectSeeOther(r, "/after-the-action")
	}, "/act")

	if status != fasthttp.StatusSeeOther {
		t.Errorf("status = %d, want 303", status)
	}
	if !strings.Contains(strings.ToLower(header), "location: /after-the-action") {
		t.Errorf("no Location header:\n%s", header)
	}
}

// A relative target must stay relative. fasthttp's own Redirect resolves it
// into an absolute URI built from the Host header and from whether this process
// terminated TLS, so an application behind a TLS-terminating proxy would answer
// an https request with Location: http://…, sending the browser to plaintext
// before the proxy sent it back. The other transport never does that.
func TestRedirectKeepsARelativeTargetRelative(t *testing.T) {
	for _, row := range []struct{ request, target, want string }{
		{"/act", "/after", "/after"},
		{"/dir/act", "sibling", "/dir/sibling"},
		{"/dir/act", "../up", "/up"},
		{"/act", "/list?page=2", "/list?page=2"},
		{"/act", "/dir/", "/dir/"},
		// An absolute target is the caller's own and passes through, because a
		// handler naming a host means that host.
		{"/act", "https://example.org/x", "https://example.org/x"},
	} {
		_, header, _ := serve(t, func(r *fasthttp.RequestCtx) {
			RedirectSeeOther(r, row.target)
		}, row.request)
		if !strings.Contains(header, "Location: "+row.want+"\r\n") {
			t.Errorf("%s + %q: want Location %q, got:\n%s", row.request, row.target, row.want, header)
		}
	}
}

// The refusal is the reason this entry exists rather than the transport being
// unportable: a redirect target is commonly a return path taken from the
// request, and the update runtime hands it to location.assign.
func TestRedirectRefusesATargetThatWouldRunScript(t *testing.T) {
	status, header, _ := serve(t, func(r *fasthttp.RequestCtx) {
		RedirectSeeOther(r, "javascript:alert(1)")
	}, "/act")

	if status != fasthttp.StatusInternalServerError {
		t.Errorf("status = %d, want 500", status)
	}
	if strings.Contains(strings.ToLower(header), "location:") {
		t.Errorf("a refused target still sent a Location:\n%s", header)
	}
}

func TestQueryValueReadsOneParameterWithoutTouchingTheRequest(t *testing.T) {
	_, _, body := serve(t, func(r *fasthttp.RequestCtx) {
		value, ok := QueryValue(r, "page")
		missing, missingOK := QueryValue(r, "absent")
		if missingOK || missing != "" {
			t.Errorf("absent parameter reported present: %q %v", missing, missingOK)
		}
		_, _ = r.WriteString(value + "|" + boolText(ok))
	}, "/list?page=3")

	if body != "3|true" {
		t.Errorf("body = %q, want %q", body, "3|true")
	}
}

func TestFormValueReadsOneSubmittedField(t *testing.T) {
	_, _, body := serveForm(t, func(r *fasthttp.RequestCtx) {
		_, _ = r.WriteString(FormValue(r, "title"))
	}, "/create", "title=hello+wave&other=1")

	if body != "hello wave" {
		t.Errorf("body = %q, want %q", body, "hello wave")
	}
}

// IsBot answers from the settings the other runtime published, because no
// configuration is bound on this transport yet.
func TestIsBotUsesTheSharedListAndThePublishedSettings(t *testing.T) {
	previous := pwruntime.ResolvedBotSettings()
	t.Cleanup(func() { pwruntime.PublishBotSettings(previous) })

	const crawler = "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
	const browser = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/140.0 Safari/537.36"

	pwruntime.PublishBotSettings(pwruntime.BotSettings{Enabled: false})
	if got := classifyThroughTransport(t, crawler); got {
		t.Error("detection is off and a crawler was still reported as one")
	}

	pwruntime.PublishBotSettings(pwruntime.BotSettings{Enabled: true})
	if got := classifyThroughTransport(t, crawler); !got {
		t.Error("a crawler was not recognised")
	}
	if got := classifyThroughTransport(t, browser); got {
		t.Error("a browser was reported as a crawler")
	}
}

func classifyThroughTransport(t *testing.T, agent string) bool {
	t.Helper()
	_, _, body := serveRaw(t, func(r *fasthttp.RequestCtx) {
		_, _ = r.WriteString(boolText(IsBot(r)))
	}, "/", "User-Agent: "+agent+"\r\n")
	return body == "true"
}

func boolText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
