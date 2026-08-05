package oidc

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
)

// federatedDocument is the shape a hostile issuer uses: the issuer field is
// honest, because that is the one value the document is checked against, and the
// endpoints point somewhere else. The token endpoint is the prize — it receives
// the authorization code and the relying party's client secret.
func federatedDocument(endpointHost string) http.RoundTripper {
	return oidcTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/.well-known/openid-configuration" {
			_, _ = io.WriteString(w, `{"issuer":"https://issuer.example",`+
				`"authorization_endpoint":"https://issuer.example/a",`+
				`"token_endpoint":"https://`+endpointHost+`/t",`+
				`"jwks_uri":"https://issuer.example/k"}`)
			return
		}
		_, _ = io.WriteString(w, `{"keys":[]}`)
	})}
}

// Naming no hosts keeps the old behaviour, because federated endpoints are
// ordinary: Google's issuer is accounts.google.com and its token endpoint is
// oauth2.googleapis.com.
func TestEndpointHostsUnsetAcceptsAFederatedEndpoint(t *testing.T) {
	_, err := Discover(context.Background(), "https://issuer.example", DiscoverOptions{
		HTTPClient: &http.Client{Transport: federatedDocument("oauth2.googleapis.com")},
	})
	if err != nil {
		t.Fatalf("an unconstrained deployment refused a federated endpoint: %v", err)
	}
}

// A deployment that named its provider's hosts gets a document pointing anywhere
// else refused, before the client secret is sent to it.
func TestEndpointHostsRefusesAHostItDidNotName(t *testing.T) {
	_, err := Discover(context.Background(), "https://issuer.example", DiscoverOptions{
		HTTPClient:    &http.Client{Transport: federatedDocument("attacker.example")},
		EndpointHosts: []string{"oauth2.googleapis.com"},
	})
	if !errors.Is(err, ErrDiscovery) {
		t.Fatalf("err = %v, want ErrDiscovery for an unnamed endpoint host", err)
	}
}

func TestEndpointHostsAcceptsAHostItNamed(t *testing.T) {
	_, err := Discover(context.Background(), "https://issuer.example", DiscoverOptions{
		HTTPClient:    &http.Client{Transport: federatedDocument("oauth2.googleapis.com")},
		EndpointHosts: []string{"oauth2.googleapis.com"},
	})
	if err != nil {
		t.Fatalf("a named endpoint host was refused: %v", err)
	}
}

// The issuer's own host never needs listing: a document pointing at the server
// that served it is the case that needed no permission.
func TestEndpointHostsAlwaysAcceptsTheIssuerHost(t *testing.T) {
	_, err := Discover(context.Background(), "https://issuer.example", DiscoverOptions{
		HTTPClient:    &http.Client{Transport: federatedDocument("issuer.example")},
		EndpointHosts: []string{"keys.example"},
	})
	if err != nil {
		t.Fatalf("the issuer's own host was refused: %v", err)
	}
}

// A rule meant to pin a host that silently admitted its subdomains would be
// worse than no rule.
func TestEndpointHostsDoesNotAdmitSubdomains(t *testing.T) {
	_, err := Discover(context.Background(), "https://issuer.example", DiscoverOptions{
		HTTPClient:    &http.Client{Transport: federatedDocument("evil.oauth2.googleapis.com")},
		EndpointHosts: []string{"oauth2.googleapis.com"},
	})
	if !errors.Is(err, ErrDiscovery) {
		t.Fatalf("err = %v, want a subdomain of a named host to be refused", err)
	}
}
