package safeurl

import "testing"

func TestABrowserMayFollowAnOrdinaryTarget(t *testing.T) {
	for _, value := range []string{
		"/dashboard",
		"/dashboard?tab=1#top",
		"dashboard",
		"./dashboard",
		"../dashboard",
		"//other.example/path",
		"http://example.com/",
		"https://example.com/path?q=1",
		"HTTPS://example.com/",
		"mailto:someone@example.com",
		"tel:+81312345678",
		// A colon inside a path, a query, or a fragment is not a scheme.
		"/a:b",
		"?next=a:b",
		"#a:b",
	} {
		if !Navigable(value) {
			t.Errorf("Navigable(%q) = false, want true", value)
		}
	}
}

func TestASchemeThatRunsScriptIsRefused(t *testing.T) {
	for _, value := range []string{
		"javascript:alert(1)",
		"JaVaScRiPt:alert(1)",
		"  javascript:alert(1)", // leading space keeps it an invalid scheme
		"java\nscript:alert(1)",
		"java\tscript:alert(1)",
		"data:text/html;base64,PHN2Zy9vbmxvYWQ9YWxlcnQoMSk+",
		"vbscript:msgbox(1)",
		"blob:https://example.com/uuid",
		"filesystem:https://example.com/temporary/x",
		"file:///etc/passwd",
		"jar:http://example.com/a.jar!/",
		"",
		":/nowhere",
		"mailto:",
		"tel:",
	} {
		if Navigable(value) {
			t.Errorf("Navigable(%q) = true, want false", value)
		}
	}
}

// A scheme is compared case-insensitively, because a browser resolves it that
// way and a comparison that did not would be bypassed by one shifted letter.
func TestSchemeComparisonIgnoresCase(t *testing.T) {
	if !Navigable("HtTp://example.com/") {
		t.Error("an uppercase http scheme should be navigable")
	}
	if Navigable("JAVASCRIPT:alert(1)") {
		t.Error("an uppercase javascript scheme should be refused")
	}
}
