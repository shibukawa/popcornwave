package pw

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/pwruntime"
)

func apiDocConfigs(ui string) (ServerConfig, SecurityConfig, MiddlewareConfig) {
	server, security, middleware, _ := validRuntimeConfigs()
	server.OpenAPI = "/openapi.json"
	server.APIDoc, server.APIDocPath = ui, "/docs"
	return server, security, middleware
}

func TestAPIDocEndpointServesConfiguredUI(t *testing.T) {
	tests := []struct {
		ui   string
		want string
	}{
		{ui: APIDocScalar, want: "@scalar/api-reference"},
		{ui: APIDocSwagger, want: "swagger-ui-bundle.js"},
	}
	for _, test := range tests {
		t.Run(test.ui, func(t *testing.T) {
			server, security, middleware := apiDocConfigs(test.ui)
			handler, err := buildRuntimeHandler(http.NotFoundHandler(), server, security, middleware, pwruntime.Resources{}, false)
			if err != nil {
				t.Fatal(err)
			}

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/docs", nil))
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d", response.Code)
			}
			if contentType := response.Header().Get("Content-Type"); contentType != "text/html; charset=utf-8" {
				t.Fatalf("content type = %q", contentType)
			}
			body := response.Body.String()
			if !strings.Contains(body, test.want) || !strings.Contains(body, `"/openapi.json"`) {
				t.Fatalf("body = %s", body)
			}

			head := httptest.NewRecorder()
			handler.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/docs", nil))
			if head.Code != http.StatusOK || head.Body.Len() != 0 {
				t.Fatalf("HEAD response = %d %q", head.Code, head.Body.String())
			}

			method := httptest.NewRecorder()
			handler.ServeHTTP(method, httptest.NewRequest(http.MethodPost, "/docs", nil))
			if method.Code != http.StatusMethodNotAllowed || method.Header().Get("Allow") != "GET, HEAD" {
				t.Fatalf("method response = %d %#v", method.Code, method.Header())
			}
		})
	}
}

func TestAPIDocEndpointDisabledByDefault(t *testing.T) {
	server, security, middleware := apiDocConfigs("")
	handler, err := buildRuntimeHandler(http.NotFoundHandler(), server, security, middleware, pwruntime.Resources{}, false)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/docs", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 while server.api_doc is empty", response.Code)
	}
}

func TestValidateAPIDocConfig(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ServerConfig)
		want   string
	}{
		{name: "unknown ui", mutate: func(s *ServerConfig) { s.APIDoc = "redoc" }, want: "server.api_doc must be"},
		{name: "needs openapi", mutate: func(s *ServerConfig) { s.OpenAPI = "" }, want: "requires server.openapi"},
		{name: "relative path", mutate: func(s *ServerConfig) { s.APIDocPath = "docs" }, want: "server.api_doc_path"},
		{name: "duplicate path", mutate: func(s *ServerConfig) { s.APIDocPath = s.Health }, want: "duplicates"},
		{name: "public overlap", mutate: func(s *ServerConfig) {
			s.Public = PublicConfig{Enabled: true, Mount: "/docs/assets"}
		}, want: "server.api_doc_path"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, _, _ := apiDocConfigs(APIDocScalar)
			test.mutate(&server)
			err := validateServerConfig(server)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}

	server, _, _ := apiDocConfigs(APIDocScalar)
	if err := validateServerConfig(server); err != nil {
		t.Fatal(err)
	}

	mux := NewServeMux()
	mux.HandleFunc("GET /docs", func(http.ResponseWriter, *http.Request) {})
	err := validateOperationalEndpointCollisions(mux, server)
	if err == nil || !strings.Contains(err.Error(), "server.api_doc_path collides") {
		t.Fatalf("error = %v", err)
	}
}

// The page loads its bundle from a CDN and initialises the UI from an inline
// script, so an application policy written for its own pages blanks it. The
// endpoint answers with the policy it needs instead, and only for itself.
func TestAPIDocReplacesTheApplicationCSPOnItsOwnResponse(t *testing.T) {
	const applicationPolicy = "default-src 'self'"
	for _, ui := range []string{APIDocScalar, APIDocSwagger} {
		t.Run(ui, func(t *testing.T) {
			server, security, middleware := apiDocConfigs(ui)
			security.Headers.ContentSecurityPolicy = applicationPolicy
			handler, err := buildRuntimeHandler(http.NotFoundHandler(), server, security, middleware, pwruntime.Resources{}, false)
			if err != nil {
				t.Fatal(err)
			}

			docs := httptest.NewRecorder()
			handler.ServeHTTP(docs, httptest.NewRequest(http.MethodGet, "/docs", nil))
			policy := docs.Header().Get("Content-Security-Policy")
			for _, want := range []string{
				"script-src 'self' https://cdn.jsdelivr.net 'unsafe-inline'",
				"style-src 'self' https://cdn.jsdelivr.net 'unsafe-inline'",
			} {
				if !strings.Contains(policy, want) {
					t.Fatalf("policy = %q, want it to contain %q", policy, want)
				}
			}

			// Every other route keeps the application's own policy; relaxing the
			// documentation page must not relax the application.
			other := httptest.NewRecorder()
			handler.ServeHTTP(other, httptest.NewRequest(http.MethodGet, "/", nil))
			if got := other.Header().Get("Content-Security-Policy"); got != applicationPolicy {
				t.Fatalf("policy on / = %q, want the application's %q", got, applicationPolicy)
			}
		})
	}
}

func TestAPIDocReplacesAReportOnlyPolicyToo(t *testing.T) {
	server, security, middleware := apiDocConfigs(APIDocScalar)
	security.Headers.ContentSecurityPolicyReportOnly = "default-src 'self'"
	handler, err := buildRuntimeHandler(http.NotFoundHandler(), server, security, middleware, pwruntime.Resources{}, false)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/docs", nil))
	if policy := response.Header().Get("Content-Security-Policy-Report-Only"); !strings.Contains(policy, "cdn.jsdelivr.net") {
		t.Fatalf("report-only policy = %q", policy)
	}
	if policy := response.Header().Get("Content-Security-Policy"); policy != "" {
		t.Fatalf("enforcing policy = %q, want none where the application set none", policy)
	}
}

// An application that configures no policy has chosen not to send one, and the
// documentation page is not the place to start.
func TestAPIDocSendsNoCSPWhenTheApplicationConfiguresNone(t *testing.T) {
	server, security, middleware := apiDocConfigs(APIDocScalar)
	handler, err := buildRuntimeHandler(http.NotFoundHandler(), server, security, middleware, pwruntime.Resources{}, false)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/docs", nil))
	for _, name := range []string{"Content-Security-Policy", "Content-Security-Policy-Report-Only"} {
		if policy := response.Header().Get(name); policy != "" {
			t.Fatalf("%s = %q, want none", name, policy)
		}
	}
}

func TestAPIDocSpecURLCannotEscapeInlineScript(t *testing.T) {
	response := httptest.NewRecorder()
	ScalarUI(`/openapi.json"</script><script>alert(1)</script>`).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/docs", nil))
	if strings.Contains(response.Body.String(), "</script><script>") {
		t.Fatalf("body = %s", response.Body.String())
	}
}
