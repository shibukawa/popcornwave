package testutil

import (
	"context"
	"fmt"
	"strings"

	"github.com/shibukawa/popcornwave/contrib/devidp"
)

// IdPInfo is what a test needs to point an application at the identity
// provider TestRun started for it.
type IdPInfo struct {
	Issuer       string
	ClientID     string
	ClientSecret string
}

// IdPOption configures the identity provider started by WithIdentityProvider.
type IdPOption func(*idpSettings) error

type idpSettings struct {
	source       string
	sourcePath   string
	users        []devidp.User
	validScopes  []string
	loginUser    string
	redirectURIs []string
	bind         func(*Config, IdPInfo)
}

// WithIdentityProvider starts a development OpenID Provider before the
// application server, so a test can drive an OIDC login without a browser and
// without external credentials.
//
// Exactly one roster source is required: WithIdPConfig, WithIdPRoster, or
// WithIdPUsers.
func WithIdentityProvider(options ...IdPOption) RunOption {
	return func(settings *runSettings) error {
		idp := &idpSettings{}
		for _, apply := range options {
			if apply == nil {
				continue
			}
			if err := apply(idp); err != nil {
				return err
			}
		}
		sources := 0
		for _, present := range []bool{idp.sourcePath != "", idp.source != "", len(idp.users) > 0} {
			if present {
				sources++
			}
		}
		if sources != 1 {
			return fmt.Errorf("testutil: WithIdentityProvider needs exactly one of WithIdPConfig, WithIdPRoster, or WithIdPUsers")
		}
		settings.idp = idp
		return nil
	}
}

// WithIdPConfig reads the roster from a devidp.toml file.
func WithIdPConfig(path string) IdPOption {
	return func(settings *idpSettings) error {
		if strings.TrimSpace(path) == "" {
			return fmt.Errorf("testutil: empty identity provider config path")
		}
		settings.sourcePath = path
		return nil
	}
}

// WithIdPRoster reads the roster from devidp.toml content held in the test.
func WithIdPRoster(source string) IdPOption {
	return func(settings *idpSettings) error {
		if strings.TrimSpace(source) == "" {
			return fmt.Errorf("testutil: empty identity provider roster")
		}
		settings.source = source
		return nil
	}
}

// WithIdPUsers builds the roster from Go values.
func WithIdPUsers(users ...devidp.User) IdPOption {
	return func(settings *idpSettings) error {
		if len(users) == 0 {
			return fmt.Errorf("testutil: no identity provider users")
		}
		settings.users = append(settings.users, users...)
		return nil
	}
}

// WithIdPScopes adds scope tokens beyond openid, profile, and email.
func WithIdPScopes(scopes ...string) IdPOption {
	return func(settings *idpSettings) error {
		settings.validScopes = append(settings.validScopes, scopes...)
		return nil
	}
}

// WithLoginUser pre-selects the subject the provider signs in as, so
// authorization redirects straight back to the application with a code.
func WithLoginUser(subject string) IdPOption {
	return func(settings *idpSettings) error {
		if strings.TrimSpace(subject) == "" {
			return fmt.Errorf("testutil: empty login subject")
		}
		settings.loginUser = subject
		return nil
	}
}

// WithIdPClient registers a client whose redirect URIs are matched exactly.
// The default client accepts any loopback callback, which is what a test
// server on a reserved port needs.
func WithIdPClient(redirectURIs ...string) IdPOption {
	return func(settings *idpSettings) error {
		if len(redirectURIs) == 0 {
			return fmt.Errorf("testutil: no redirect URIs")
		}
		settings.redirectURIs = append(settings.redirectURIs, redirectURIs...)
		return nil
	}
}

// WithIdPBinding writes the issuer and generated client credentials into the
// copied configuration. It runs after customize, so it wins over a placeholder
// the test left there.
func WithIdPBinding(bind func(*Config, IdPInfo)) IdPOption {
	return func(settings *idpSettings) error {
		if bind == nil {
			return fmt.Errorf("testutil: nil identity provider binding")
		}
		settings.bind = bind
		return nil
	}
}

// IdP returns the running provider, or nil when the test did not start one.
func (server *Server) IdP() *devidp.Server { return server.idp }

// IdPInfo returns the issuer and generated client credentials.
func (server *Server) IdPInfo() IdPInfo { return server.idpInfo }

// LoginAs changes the pre-selected subject for the next authorization.
func (server *Server) LoginAs(t TestingT, subject string) {
	t.Helper()
	if server.idp == nil {
		t.Fatalf("testutil: no identity provider; add WithIdentityProvider")
		return
	}
	if err := server.idp.SetLoginUser(subject); err != nil {
		t.Fatalf("testutil: select identity provider user: %v", err)
	}
}

// startIdentityProvider resolves the roster, starts the provider, registers a
// client for this run, and applies the configuration binding.
func startIdentityProvider(settings *idpSettings, config *Config) (*devidp.Server, IdPInfo, error) {
	roster, err := resolveRoster(settings)
	if err != nil {
		return nil, IdPInfo{}, err
	}
	server, err := devidp.Start(context.Background(), "127.0.0.1:0", roster, devidp.Options{
		LoginUser: settings.loginUser,
	})
	if err != nil {
		return nil, IdPInfo{}, err
	}
	credentials, err := server.RegisterClient(devidp.ClientSpec{
		RedirectURIs:      settings.redirectURIs,
		LoopbackRedirects: len(settings.redirectURIs) == 0,
	})
	if err != nil {
		_ = server.Close()
		return nil, IdPInfo{}, err
	}
	info := IdPInfo{Issuer: server.Issuer(), ClientID: credentials.ID, ClientSecret: credentials.Secret}
	if settings.bind != nil {
		settings.bind(config, info)
	}
	return server, info, nil
}

func resolveRoster(settings *idpSettings) (devidp.Config, error) {
	switch {
	case settings.sourcePath != "":
		roster, err := devidp.LoadConfig(settings.sourcePath)
		if err != nil {
			return devidp.Config{}, err
		}
		roster.ValidScopes = append(roster.ValidScopes, settings.validScopes...)
		return roster, nil
	case settings.source != "":
		roster, err := devidp.ParseConfig([]byte(settings.source), ".")
		if err != nil {
			return devidp.Config{}, err
		}
		roster.ValidScopes = append(roster.ValidScopes, settings.validScopes...)
		return roster, nil
	default:
		return devidp.Config{
			Users:       settings.users,
			ValidScopes: settings.validScopes,
		}, nil
	}
}
