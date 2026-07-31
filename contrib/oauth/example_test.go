package oauth_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/shibukawa/popcornwave/authstate/memory"
	"github.com/shibukawa/popcornwave/contrib/oauth"
)

func ExampleNewClient() {
	store, err := memory.NewStore[oauth.Transaction](memory.Options{})
	if err != nil {
		panic(err)
	}
	client, err := oauth.NewClient(oauth.Config{
		AuthorizationEndpoint: "https://issuer.example/authorize",
		TokenEndpoint:         "https://issuer.example/token",
		ClientID:              "client",
		ClientSecret:          "secret",
		RedirectURI:           "https://app.example/callback",
		EndpointValidator: func(endpoint *url.URL) error {
			if endpoint.Scheme != "https" {
				return errors.New("endpoint must use HTTPS")
			}
			return nil
		},
	}, oauth.Options{
		StateStore: store,
		TransactionValidator: func(transaction oauth.Transaction) error {
			if transaction.State == "" {
				return errors.New("missing state")
			}
			return nil
		},
	})
	if err != nil {
		panic(err)
	}
	authorizationURL, transactionKey, err := client.BeginAuthorization(context.Background(), oauth.BeginOptions{Scopes: []string{"profile"}})
	if err != nil {
		panic(err)
	}
	fmt.Println(strings.HasPrefix(authorizationURL, "https://"), transactionKey != "")
	// Output: true true
}
