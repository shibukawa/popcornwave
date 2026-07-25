package pw

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	kzstd "github.com/klauspost/compress/zstd"
	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinybind-go/configbind"
	"github.com/shibukawa/tinybind-go/htmlbind"
)

func TestScaffoldsIncludeBuiltInDefinitions(t *testing.T) {
	toml, err := ScaffoldTOML()
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"[server]", "port = 8080", `read_header_timeout = "5s"`,
		`health.path = "/healthz"`, `readiness.path = "/readyz"`,
		`public.enabled = true`, `public.mount = "/public"`,
		`headers.frame_options = "deny"`, "[observability]", "[middleware]",
		"access_log = true",
	} {
		if !strings.Contains(toml, fragment) {
			t.Fatalf("TOML scaffold missing %q:\n%s", fragment, toml)
		}
	}
	env, err := ScaffoldEnv()
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"PORT=8080", "SERVER_MAX_REQUEST_BODY=10485760",
		"SERVER_HEALTH_ENABLED=true", "SECURITY_HEADERS_ENABLED=true",
		"OTEL_SERVICE_NAME=\"\"", "SESSION_SECRET=\"\"",
	} {
		if !strings.Contains(env, fragment) {
			t.Fatalf("env scaffold missing %q:\n%s", fragment, env)
		}
	}
}

func TestMiddlewaresParseAndInjectConfiguration(t *testing.T) {
	SetConfigLoadOptions(configbind.LoadOptions{
		Vendor: "popcornwave-test", Tool: "pw-test", FileName: "missing.toml",
		Args: []string{"--port", "9090"}, Environ: []string{"PORT=7070"},
	})
	handler, err := Middlewares(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		config := Config[ServerConfig](r.Context())
		if config.Port != 9090 {
			t.Errorf("Port = %d", config.Port)
		}
		if Logger(r.Context()) == nil {
			t.Error("nil logger")
		}
		w.WriteHeader(http.StatusNoContent)
	}), WithPublicFS(fstest.MapFS{".keep": {Data: nil}}))
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d", recorder.Code)
	}
	if recorder.Header().Get("X-Request-ID") == "" {
		t.Fatal("request ID was not added")
	}
}

func TestWriteHTMLBuffersAndWrites(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	builder := htmlbind.Builder[string]{}
	leaf := htmlbind.Bind(&htmlbind.Plan[string]{Ops: []htmlbind.Op[string]{
		builder.Static("<h1>"),
		builder.Text(func(value string) string { return value }),
		builder.Static("</h1>"),
	}}, "Hello")
	WriteHTML(recorder, request, leaf)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "<h1>Hello</h1>" {
		t.Fatalf("response = %d %q", recorder.Code, recorder.Body.String())
	}
}

func TestWriteHTMLChainNestsWrappers(t *testing.T) {
	type documentParams struct {
		Children htmlbind.Fragment
	}
	documentBuilder := htmlbind.Builder[documentParams]{}
	documentPlan := &htmlbind.Plan[documentParams]{Ops: []htmlbind.Op[documentParams]{
		documentBuilder.Static("<!doctype html><html><body>"),
		documentBuilder.Slot(func(params documentParams) htmlbind.Fragment { return params.Children }, nil),
		documentBuilder.Static("</body></html>"),
	}}
	document := htmlbind.BindWrapper(documentPlan, documentParams{}, func(params *documentParams, children htmlbind.Fragment) {
		params.Children = children
	})
	pageBuilder := htmlbind.Builder[struct{}]{}
	page := htmlbind.Bind(&htmlbind.Plan[struct{}]{Ops: []htmlbind.Op[struct{}]{
		pageBuilder.Static("<main>page</main>"),
	}}, struct{}{})

	recorder := httptest.NewRecorder()
	WriteHTMLChain(recorder, httptest.NewRequest(http.MethodGet, "/", nil), []htmlbind.Wrapper{document}, page)
	if recorder.Code != http.StatusOK ||
		recorder.Body.String() != "<!doctype html><html><body><main>page</main></body></html>" {
		t.Fatalf("response = %d %q", recorder.Code, recorder.Body.String())
	}
}

func TestWriteHTMLPreservesConfiguredZstdCompression(t *testing.T) {
	builder := htmlbind.Builder[struct{}]{}
	leaf := htmlbind.Bind(&htmlbind.Plan[struct{}]{Ops: []htmlbind.Op[struct{}]{
		builder.Static("<main>compressed</main>"),
	}}, struct{}{})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Accept-Encoding", "zstd")
	request = request.WithContext(pwruntime.WithResources(request.Context(), pwruntime.Resources{
		Configs: map[reflect.Type]any{
			reflect.TypeFor[MiddlewareConfig](): MiddlewareConfig{Compression: true},
		},
	}))
	recorder := httptest.NewRecorder()

	WriteHTML(recorder, request, leaf)

	if recorder.Header().Get("Content-Encoding") != "zstd" {
		t.Fatalf("Content-Encoding = %q", recorder.Header().Get("Content-Encoding"))
	}
	decoder, err := kzstd.NewReader(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer decoder.Close()
	body, err := decoder.DecodeAll(recorder.Body.Bytes(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "<main>compressed</main>" {
		t.Fatalf("decoded body = %q", body)
	}
}

func TestUnsupportedStreamAcceptWrites406(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Accept", "application/xml")
	stream := NewStream[map[string]string](recorder, request)
	if err := stream.Send(map[string]string{"value": "ignored"}); err == nil {
		t.Fatal("Send returned nil after negotiation failure")
	}
	if recorder.Code != http.StatusNotAcceptable || !bytes.Contains(recorder.Body.Bytes(), []byte(`"code":"not_acceptable"`)) {
		t.Fatalf("response = %d %q", recorder.Code, recorder.Body.String())
	}
}
