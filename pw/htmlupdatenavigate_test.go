package pw

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The browser hands a navigate directive to location.assign, which runs a
// javascript: URL rather than going anywhere. The target commonly starts life as
// a return path taken from the request, so refusing it here is what keeps an
// application from turning its own redirect into script execution.
func TestANavigationTargetThatWouldRunScriptIsRefused(t *testing.T) {
	for _, target := range []string{
		"javascript:alert(1)",
		"JaVaScRiPt:alert(1)",
		"data:text/html;base64,PHN2Zy9vbmxvYWQ9YWxlcnQoMSk+",
		"vbscript:msgbox(1)",
		"",
	} {
		recorder := httptest.NewRecorder()
		WriteUpdateNavigate(recorder, httptest.NewRequest(http.MethodPost, "/orders", nil), target)

		if recorder.Code != http.StatusInternalServerError {
			t.Errorf("WriteUpdateNavigate(%q) status = %d, want %d", target, recorder.Code, http.StatusInternalServerError)
		}
		if body := recorder.Body.String(); strings.Contains(body, target) && target != "" {
			t.Errorf("WriteUpdateNavigate(%q) echoed the target into the response: %s", target, body)
		}
		if strings.Contains(recorder.Body.String(), `"navigate"`) {
			t.Errorf("WriteUpdateNavigate(%q) still emitted a navigate directive", target)
		}
	}
}

// The refusal is narrow: an ordinary target still leaves the page. A rule that
// also refused these would be reported as the framework being broken rather than
// as the application being wrong.
func TestAnOrdinaryNavigationTargetStillPasses(t *testing.T) {
	for _, target := range []string{
		"/orders/17",
		"https://example.com/orders/17",
		"mailto:support@example.com",
	} {
		recorder := httptest.NewRecorder()
		WriteUpdateNavigate(recorder, httptest.NewRequest(http.MethodPost, "/orders", nil), target)

		if recorder.Code == http.StatusInternalServerError {
			t.Errorf("WriteUpdateNavigate(%q) was refused: %s", target, recorder.Body.String())
		}
	}
}
