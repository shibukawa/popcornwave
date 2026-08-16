package pw

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A loader has no writer, so its only way to send the browser elsewhere is to
// return a value. The response path has to recognise it.
func TestAReturnedRedirectBecomesARedirectResponse(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/account", nil)

	WriteProblem(recorder, request, SeeOther("/auth/login"))

	if recorder.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != "/auth/login" {
		t.Errorf("Location = %q, want /auth/login", location)
	}
}

// One constructor per code, so none can drift from the status its name claims.
func TestEachRedirectFormCarriesItsStatus(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		status int
	}{
		{"see other", SeeOther("/next"), http.StatusSeeOther},
		{"temporary keeping method", TemporaryRedirect("/next"), http.StatusTemporaryRedirect},
		{"moved permanently", MovedPermanently("/next"), http.StatusMovedPermanently},
		{"permanent", PermanentRedirect("/next"), http.StatusPermanentRedirect},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			WriteProblem(recorder, httptest.NewRequest(http.MethodGet, "/", nil), test.err)
			if recorder.Code != test.status {
				t.Errorf("status = %d, want %d", recorder.Code, test.status)
			}
		})
	}
}

// The safety check belongs to the write rather than to the value, so a target
// a browser could only follow by running script is refused on this path too.
func TestAReturnedRedirectToScriptIsRefused(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	WriteProblem(recorder, request, SeeOther("javascript:alert(1)"))

	if recorder.Code == http.StatusSeeOther {
		t.Fatalf("a javascript: target was sent as a redirect")
	}
	if location := recorder.Header().Get("Location"); location != "" {
		t.Errorf("Location = %q, want none", location)
	}
}

// A returned redirect wrapped by a caller still redirects, because the response
// path reads it with errors.As rather than by equality.
func TestAWrappedReturnedRedirectStillRedirects(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	WriteProblem(recorder, request, fmt.Errorf("loading the page: %w", SeeOther("/next")))

	if recorder.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", recorder.Code)
	}
}

// An ordinary problem must keep answering as one; recognising a redirect must
// not swallow everything else that reaches this path.
func TestAProblemIsStillAProblem(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	WriteProblem(recorder, request, NotFound("no such user"))

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", recorder.Code)
	}
}
