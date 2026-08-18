package middlewares

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/shibukawa/popcornweb/pwruntime"
)

type resourceConfig struct{ Port int }

func TestInjectResourcesPublishesRuntimeResources(t *testing.T) {
	sink := pwruntime.NewCaptureSink()
	resources := pwruntime.Resources{
		Configs: map[reflect.Type]any{reflect.TypeFor[resourceConfig](): resourceConfig{Port: 9090}},
		Log:     pwruntime.NewLogBackend(pwruntime.LevelInfo, sink),
	}
	var (
		config resourceConfig
		found  bool
	)
	InjectResources(resources)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		config, found = pwruntime.Config[resourceConfig](r.Context())
		pwruntime.ReadLogger(r.Context()).Info("published")
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if !found || config.Port != 9090 {
		t.Fatalf("config = %#v, found = %v", config, found)
	}
	if records := sink.Records(); len(records) != 1 || records[0].Message != "published" {
		t.Fatalf("the published backend did not receive the record: %#v", records)
	}
}
