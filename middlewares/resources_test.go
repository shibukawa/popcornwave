package middlewares

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/shibukawa/popcornwave/pwruntime"
)

type resourceConfig struct{ Port int }

func TestInjectResourcesPublishesRuntimeResources(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	resources := pwruntime.Resources{
		Configs: map[reflect.Type]any{reflect.TypeFor[resourceConfig](): resourceConfig{Port: 9090}},
		Logger:  logger,
	}
	var (
		config resourceConfig
		found  bool
		bound  *slog.Logger
	)
	InjectResources(resources)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		config, found = pwruntime.Config[resourceConfig](r.Context())
		bound = pwruntime.Logger(r.Context())
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if !found || config.Port != 9090 {
		t.Fatalf("config = %#v, found = %v", config, found)
	}
	if bound != logger {
		t.Fatal("request logger was not published")
	}
}
