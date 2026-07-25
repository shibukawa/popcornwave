package middlewares

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMaxRequestBodyRejectsOversizedBody(t *testing.T) {
	var readErr error
	handler := MaxRequestBody(4)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	}))
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("too large"))
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if readErr == nil {
		t.Fatal("oversized body was accepted")
	}
}

func TestMaxRequestBodyWithoutLimitPassesThrough(t *testing.T) {
	var body string
	handler := MaxRequestBody(0)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		read, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read failed: %v", err)
		}
		body = string(read)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader("unbounded")))
	if body != "unbounded" {
		t.Fatalf("body = %q", body)
	}
}
