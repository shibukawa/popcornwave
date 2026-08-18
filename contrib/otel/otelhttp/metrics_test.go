package otelhttp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shibukawa/popcornweb/contrib/otel/metric"
)

func TestClientInstrumentsRecordTheCalleeAndNotThePath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	provider := metric.NewProvider()
	client := NewClient(nil, WithMeterProvider(provider))
	// A path-per-object URL, so the assertion that no path reached an attribute
	// is about the case that would actually hurt.
	response, err := client.Get(server.URL + "/objects/4711")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()

	collected := provider.Collect()
	var duration metric.Data
	var found bool
	for _, scope := range collected.Scopes {
		for _, data := range scope.Metrics {
			if data.Name == "http.client.request.duration" {
				duration, found = data, true
			}
		}
	}
	if !found {
		t.Fatal("the client duration instrument recorded nothing")
	}
	if len(duration.Histograms) != 1 {
		t.Fatalf("series = %d, want one", len(duration.Histograms))
	}
	keys := map[string]bool{}
	for _, attribute := range duration.Histograms[0].Attributes {
		keys[attribute.Key] = true
		if value, ok := attribute.Value.AsString(); ok && value == "/objects/4711" {
			t.Errorf("the request path reached attribute %s", attribute.Key)
		}
	}
	for _, want := range []string{"http.request.method", "server.address", "http.response.status_code"} {
		if !keys[want] {
			t.Errorf("attribute %s is missing", want)
		}
	}
	if keys["url.path"] {
		t.Error("url.path is an attribute, which is one series per object")
	}
}

func TestClientInstrumentsAreOptIn(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()
	provider := metric.NewProvider()
	// No WithMeterProvider: a client this framework did not build records
	// nothing rather than reaching for the process provider.
	response, err := NewClient(nil).Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if scopes := provider.Collect().Scopes; len(scopes) != 0 {
		t.Fatalf("an uninstrumented client recorded %d scopes", len(scopes))
	}
}
