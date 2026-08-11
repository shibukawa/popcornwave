package pwfast

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shibukawa/tinygodriver/fasthttp"
)

func TestNetHTTPHandlerPreservesTheHTTPContract(t *testing.T) {
	handler := NetHTTPHandler(func(ctx *fasthttp.RequestCtx) {
		if string(ctx.Method()) != http.MethodPost || string(ctx.RequestURI()) != "/items?a=1&a=2" {
			ctx.Error("request target", fasthttp.StatusBadRequest)
			return
		}
		if string(ctx.Request.Header.Peek("X-Test")) != "one" || string(ctx.PostBody()) != "binary\x00body" {
			ctx.Error("request data", fasthttp.StatusBadRequest)
			return
		}
		ctx.Response.Header.Add("Set-Cookie", "a=1")
		ctx.Response.Header.Add("Set-Cookie", "b=2")
		ctx.Response.Header.Add("X-Value", "first")
		ctx.Response.Header.Add("X-Value", "second")
		ctx.SetStatusCode(fasthttp.StatusCreated)
		ctx.SetBody([]byte{'o', 'k', 0, 255})
	})

	request := httptest.NewRequest(http.MethodPost, "http://example.test/items?a=1&a=2", bytes.NewBufferString("binary\x00body"))
	request.Header.Add("X-Test", "one")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d", response.StatusCode)
	}
	if got := response.Header.Values("Set-Cookie"); len(got) != 2 || got[0] != "a=1" || got[1] != "b=2" {
		t.Errorf("Set-Cookie = %v", got)
	}
	if got := response.Header.Values("X-Value"); len(got) != 2 {
		t.Errorf("X-Value = %v", got)
	}
	if got := recorder.Body.Bytes(); !bytes.Equal(got, []byte{'o', 'k', 0, 255}) {
		t.Errorf("body = %v", got)
	}
}

func TestNetHTTPHandlerRejectsNil(t *testing.T) {
	recorder := httptest.NewRecorder()
	NetHTTPHandler(nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d", recorder.Code)
	}
}
