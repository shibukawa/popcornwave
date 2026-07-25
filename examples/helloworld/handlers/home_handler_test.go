package handlers

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/pw"
	"github.com/shibukawa/popcornwave/testutil"
	_ "helloworld"
	_ "helloworld/templates"
)

func TestHomeRendersNestedDocumentAndIncrementsCounter(t *testing.T) {
	server := testutil.TestRun(t, Handlers(), func(config *testutil.Config) {
		testutil.Update[pw.MiddlewareConfig](config, func(middleware *pw.MiddlewareConfig) {
			middleware.RDB = pw.RDBConfig{
				Enabled:         true,
				DSN:             "sqlite://:memory:",
				AutoTransaction: false,
				ConnectTimeout:  time.Second,
				MaxOpenConns:    1,
				MaxIdleConns:    1,
			}
		})
	}, testutil.WithMigrations("../migrations"))

	for visit, expected := range []string{">1</strong>", ">2</strong>"} {
		response, err := server.Client().Get(server.URL + "/")
		if err != nil {
			t.Fatal(err)
		}
		bodyBytes, err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusOK {
			t.Fatalf("visit %d status = %d body=%q", visit+1, response.StatusCode, bodyBytes)
		}
		body := string(bodyBytes)
		for _, fragment := range []string{
			"<!doctype html>",
			"<title>Hello World · Popcorn Wave</title>",
			"Hello, ",
			"World!",
			expected,
		} {
			if !strings.Contains(body, fragment) {
				t.Fatalf("visit %d body is missing %q: %s", visit+1, fragment, body)
			}
		}
	}
}
