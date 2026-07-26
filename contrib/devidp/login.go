package devidp

import (
	"encoding/json"
	"html/template"
	"net/http"
	"sort"
	"strings"
)

// loginTemplate renders the selection screen. There is no credential field on
// this page by design; the provider proves nothing beyond "a developer picked
// this identity".
var loginTemplate = template.Must(template.New("login").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Select a development user</title>
<style>
:root { color-scheme: light dark; }
body { font: 16px/1.5 system-ui, sans-serif; margin: 0; padding: 2rem 1rem; }
main { max-width: 40rem; margin: 0 auto; }
.banner { background: #b45309; color: #fff; padding: .75rem 1rem; border-radius: .5rem; font-weight: 600; }
.context { color: #6b7280; margin: 1rem 0; font-size: .9rem; }
.message { color: #b91c1c; }
ul { list-style: none; padding: 0; margin: 0; }
li { border: 1px solid #9ca3af55; border-radius: .5rem; margin-bottom: .75rem; }
button { all: unset; display: block; width: 100%; box-sizing: border-box; padding: 1rem; cursor: pointer; }
button:hover, button:focus { background: #2563eb22; }
.name { font-weight: 600; }
.subject { color: #6b7280; font-size: .85rem; }
.claims { color: #6b7280; font-size: .8rem; margin-top: .35rem; word-break: break-all; }
.cancel { margin-top: 1.5rem; color: #6b7280; font-size: .9rem; }
.cancel button { display: inline; width: auto; padding: 0; text-decoration: underline; }
</style>
</head>
<body>
<main>
<p class="banner">Development identity provider &mdash; no password is checked</p>
<p class="context">Client <code>{{.ClientID}}</code> requested <code>{{.Scopes}}</code>.</p>
{{if .Message}}<p class="message">{{.Message}}</p>{{end}}
<form method="post" action="{{.Action}}">
<input type="hidden" name="auth" value="{{.Auth}}">
<input type="hidden" name="csrf" value="{{.CSRF}}">
<ul>
{{range .Users}}
<li><button type="submit" name="subject" value="{{.Subject}}">
<span class="name">{{.DisplayName}}</span>
<span class="subject">&mdash; {{.Subject}}</span>
{{if .Claims}}<div class="claims">{{.Claims}}</div>{{end}}
</button></li>
{{end}}
</ul>
<p class="cancel"><button type="submit" name="cancel" value="1">Cancel and return to the application</button></p>
</form>
</main>
</body>
</html>
`))

var errorTemplate = template.Must(template.New("error").Parse(`<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>Development identity provider</title>
<style>body { font: 16px/1.5 system-ui, sans-serif; margin: 0; padding: 2rem 1rem; }
main { max-width: 40rem; margin: 0 auto; } .banner { background: #b45309; color: #fff; padding: .75rem 1rem; border-radius: .5rem; font-weight: 600; }</style>
</head>
<body><main>
<p class="banner">Development identity provider &mdash; no password is checked</p>
<p>{{.}}</p>
</main></body>
</html>
`))

type loginUserView struct {
	Subject     string
	DisplayName string
	Claims      string
}

type loginView struct {
	Action   string
	Auth     string
	CSRF     string
	ClientID string
	Scopes   string
	Message  string
	Users    []loginUserView
}

func (p *Provider) renderLogin(w http.ResponseWriter, r *http.Request, key string, pending *pendingAuthorization, message string) {
	view := loginView{
		Action:   p.loginPath(),
		Auth:     key,
		CSRF:     pending.csrf,
		ClientID: pending.clientID,
		Scopes:   strings.Join(pending.scopes, " "),
		Message:  message,
	}
	for _, user := range p.Users() {
		view.Users = append(view.Users, loginUserView{
			Subject:     user.Subject,
			DisplayName: user.DisplayName,
			Claims:      summarizeClaims(user.Claims),
		})
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	_ = loginTemplate.Execute(w, view)
}

func (p *Provider) renderError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = errorTemplate.Execute(w, message)
}

// loginPath returns the form action relative to the served host.
func (p *Provider) loginPath() string {
	trimmed := strings.TrimPrefix(p.issuer, "http://")
	trimmed = strings.TrimPrefix(trimmed, "https://")
	if index := strings.IndexByte(trimmed, '/'); index >= 0 {
		return strings.TrimSuffix(trimmed[index:], "/") + "/login"
	}
	return "/login"
}

// summarizeClaims renders roster claims as a stable one-line preview.
func summarizeClaims(claims map[string]any) string {
	if len(claims) == 0 {
		return ""
	}
	names := make([]string, 0, len(claims))
	for name := range claims {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		encoded, err := json.Marshal(claims[name])
		if err != nil {
			continue
		}
		parts = append(parts, name+"="+string(encoded))
	}
	return strings.Join(parts, "  ")
}
