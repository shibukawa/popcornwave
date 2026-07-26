package pw

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func enabledAuth() AuthConfig {
	return AuthConfig{
		Enabled: true, Mode: AuthModeOIDC,
		LoginPath: DefaultLoginPath, CallbackPath: DefaultCallbackPath, LogoutPath: DefaultLogoutPath,
		PostLoginRedirect: "/", PostLogoutRedirect: "/",
	}
}

func stubHandlers(recorder *[]string) AuthHandlers {
	mark := func(name string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*recorder = append(*recorder, name)
			_, _ = io.WriteString(w, name)
		})
	}
	return AuthHandlers{Login: mark("login"), Callback: mark("callback"), Logout: mark("logout")}
}

func TestAuthEndpointsRouteByPathAndMethod(t *testing.T) {
	var called []string
	application := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "application")
	})
	handler := authEndpoints(application, enabledAuth(), stubHandlers(&called))

	for _, testCase := range []struct {
		method string
		path   string
		status int
		body   string
	}{
		{http.MethodGet, DefaultLoginPath, http.StatusOK, "login"},
		{http.MethodGet, DefaultCallbackPath, http.StatusOK, "callback"},
		{http.MethodPost, DefaultLogoutPath, http.StatusOK, "logout"},
		{http.MethodGet, "/", http.StatusOK, "application"},
		{http.MethodPost, DefaultLoginPath, http.StatusMethodNotAllowed, ""},
		{http.MethodPost, DefaultCallbackPath, http.StatusMethodNotAllowed, ""},
		// A logout must not be reachable by navigation or prefetch.
		{http.MethodGet, DefaultLogoutPath, http.StatusMethodNotAllowed, ""},
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(testCase.method, "http://example.test"+testCase.path, nil))
		if recorder.Code != testCase.status {
			t.Fatalf("%s %s status = %d, want %d", testCase.method, testCase.path, recorder.Code, testCase.status)
		}
		if testCase.body != "" && !strings.Contains(recorder.Body.String(), testCase.body) {
			t.Fatalf("%s %s body = %q, want %q", testCase.method, testCase.path, recorder.Body.String(), testCase.body)
		}
		if testCase.status == http.StatusMethodNotAllowed && recorder.Header().Get("Allow") == "" {
			t.Fatalf("%s %s has no Allow header", testCase.method, testCase.path)
		}
	}
}

func TestInstallAuthEndpointsRequiresAProvider(t *testing.T) {
	authProviderState.Lock()
	previous := authProviderState.provider
	authProviderState.provider = nil
	authProviderState.Unlock()
	t.Cleanup(func() {
		authProviderState.Lock()
		authProviderState.provider = previous
		authProviderState.Unlock()
	})

	_, err := installAuthEndpoints(http.NotFoundHandler(), authRuntime{auth: enabledAuth()})
	if err == nil || !strings.Contains(err.Error(), "popcornwave/auth") {
		t.Fatalf("err = %v, want a hint to import the authentication package", err)
	}

	// Disabled authentication needs no provider and changes no handler.
	handler, err := installAuthEndpoints(http.NotFoundHandler(), authRuntime{})
	if err != nil || handler == nil {
		t.Fatalf("handler = %v, err = %v", handler, err)
	}
}

func TestInstallAuthEndpointsAppliesResolve(t *testing.T) {
	RegisterAuthProvider(func(AuthConfig, SessionConfig) (AuthHandlers, error) {
		var called []string
		handlers := stubHandlers(&called)
		handlers.Resolve = func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), Identity{Subject: "resolved"})))
			})
		}
		return handlers, nil
	})
	t.Cleanup(func() {
		authProviderState.Lock()
		authProviderState.provider = nil
		authProviderState.Unlock()
	})

	handler, err := installAuthEndpoints(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := CurrentUser(r.Context())
		if !ok {
			http.Error(w, "anonymous", http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, identity.Subject)
	}), authRuntime{auth: enabledAuth()})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://example.test/", nil))
	if recorder.Body.String() != "resolved" {
		t.Fatalf("body = %q", recorder.Body.String())
	}
}

func TestCurrentUserReadsTheInstalledIdentity(t *testing.T) {
	if _, ok := CurrentUser(t.Context()); ok {
		t.Fatal("an empty context must carry no identity")
	}
	ctx := WithIdentity(t.Context(), Identity{Subject: "admin", Claims: map[string]string{"role": "admin"}})
	identity, ok := CurrentUser(ctx)
	if !ok || identity.Subject != "admin" {
		t.Fatalf("identity = %+v, ok = %v", identity, ok)
	}
	if role, ok := identity.Claim("role"); !ok || role != "admin" {
		t.Fatalf("role = %q, ok = %v", role, ok)
	}
	if _, ok := identity.Claim("missing"); ok {
		t.Fatal("an absent claim must report false")
	}
}
