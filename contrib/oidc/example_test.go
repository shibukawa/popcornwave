package oidc_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/shibukawa/popcornweb/contrib/oidc"
)

type exampleDiscoveryTransport struct{}

func (exampleDiscoveryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body := `{"issuer":"https://issuer.example","authorization_endpoint":"https://issuer.example/authorize","token_endpoint":"https://issuer.example/token","jwks_uri":"https://issuer.example/keys"}`
	if strings.HasSuffix(req.URL.Path, "/keys") {
		body = `{"keys":[]}`
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

func ExampleDiscover() {
	provider, err := oidc.Discover(context.Background(), "https://issuer.example", oidc.DiscoverOptions{
		HTTPClient: &http.Client{Transport: exampleDiscoveryTransport{}},
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(provider != nil)
	// Output: true
}
