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

func TestAPIDocSpecURLCannotEscapeInlineScript(t *testing.T) {
	response := httptest.NewRecorder()
	ScalarUI(`/openapi.json"</script><script>alert(1)</script>`).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/docs", nil))
	if strings.Contains(response.Body.String(), "</script><script>") {
		t.Fatalf("body = %s", response.Body.String())
	}
}
