package pwcli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shibukawa/popcornwave/contrib/devidp"
)

// Environment variables the development identity provider injects into the
// application process. They outrank TOML in the configuration precedence, so a
// project needs no issuer or client credentials in any file it commits.
const (
	envIdPIssuer       = "AUTH_OIDC_ISSUER"
	envIdPClientID     = "AUTH_OIDC_CLIENT_ID"
	envIdPClientSecret = "AUTH_OIDC_CLIENT_SECRET"
)

// devIdentityProvider is the running provider plus the values pw dev injects.
type devIdentityProvider struct {
	server *devidp.Server
	path   string
	env    []string
}

// startDevIdentityProvider runs the development provider and registers one
// ephemeral client for this pw dev session.
func startDevIdentityProvider(ctx context.Context, root string, config projectConfig, stdout io.Writer) (*devIdentityProvider, error) {
	path := filepath.Join(root, filepath.FromSlash(config.IdP.Config))
	roster, err := devidp.LoadConfig(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w\n%s", err, idpRosterSample(config.IdP.Config))
		}
		return nil, err
	}
	address := "127.0.0.1:" + strconv.Itoa(config.IdP.Port)
	server, err := devidp.Start(ctx, address, roster, devidp.Options{
		Logf: func(format string, args ...any) { fmt.Fprintf(stdout, format+"\n", args...) },
	})
	if err != nil {
		return nil, err
	}
	credentials, err := server.RegisterClient(devidp.ClientSpec{LoopbackRedirects: true})
	if err != nil {
		_ = server.Close()
		return nil, err
	}
	provider := &devIdentityProvider{
		server: server,
		path:   path,
		env: []string{
			envIdPIssuer + "=" + server.Issuer(),
			envIdPClientID + "=" + credentials.ID,
			envIdPClientSecret + "=" + credentials.Secret,
		},
	}
	provider.report(stdout)
	return provider, nil
}

// report prints what the application was handed. The client secret is masked
// because it reaches the application through the environment, not the terminal.
func (p *devIdentityProvider) report(stdout io.Writer) {
	fmt.Fprintf(stdout, "pw dev: identity provider %s\n", p.server.Issuer())
	fmt.Fprintf(stdout, "pw dev:   login screen %s\n", p.server.Endpoint("/login"))
	fmt.Fprintf(stdout, "pw dev:   client %s (secret injected as %s)\n", p.clientID(), envIdPClientSecret)
	subjects := make([]string, 0, 8)
	for _, user := range p.server.Users() {
		subjects = append(subjects, user.Subject)
	}
	fmt.Fprintf(stdout, "pw dev:   users %s\n", strings.Join(subjects, ", "))
}

func (p *devIdentityProvider) clientID() string {
	for _, entry := range p.env {
		if value, ok := strings.CutPrefix(entry, envIdPClientID+"="); ok {
			return value
		}
	}
	return ""
}

// reload applies an edited roster without restarting the provider, so the
// issuer and the injected credentials the application already holds stay valid.
func (p *devIdentityProvider) reload(stdout, stderr io.Writer) {
	roster, err := devidp.LoadConfig(p.path)
	if err != nil {
		fmt.Fprintln(stderr, "pw dev:", err)
		return
	}
	if err := p.server.Reload(roster); err != nil {
		fmt.Fprintln(stderr, "pw dev:", err)
		return
	}
	fmt.Fprintf(stdout, "pw dev: reloaded %s\n", filepath.Base(p.path))
}

// watchState reports the roster file size and modification time so pw dev can
// notice an edit without adding the file to the rebuild watch set.
func (p *devIdentityProvider) watchState() fileState {
	if p == nil {
		return fileState{}
	}
	info, err := os.Stat(p.path)
	if err != nil {
		return fileState{}
	}
	return fileState{size: info.Size(), modTime: info.ModTime()}
}

func (p *devIdentityProvider) close() {
	if p != nil && p.server != nil {
		_ = p.server.Close()
	}
}

// environ returns the process environment for the application, preserving any
// value the developer already exported.
func (p *devIdentityProvider) environ(base []string) []string {
	if p == nil {
		return base
	}
	for _, entry := range p.env {
		name, _, _ := strings.Cut(entry, "=")
		if value, ok := os.LookupEnv(name); ok && strings.TrimSpace(value) != "" {
			continue
		}
		base = append(base, entry)
	}
	return base
}

func idpRosterSample(name string) string {
	return "pw dev: create " + name + " to list the development users, for example:\n\n" +
		"    [users.admin]\n" +
		"    display_name = \"Administrator\"\n" +
		"    [users.admin.claims]\n" +
		"    email = \"admin@example.com\"\n" +
		"    role = \"admin\"\n"
}
