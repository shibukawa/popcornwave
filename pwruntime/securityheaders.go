package pwruntime

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// SecurityHeadersConfig contains browser security response headers.
//
// Enabled records whether a runtime selects this middleware at all; the
// middleware itself always applies the configured headers.
//
// It lives in the shared leaf because the headers a browser is sent should not
// depend on which transport served the response, and because everything about
// resolving them — the validation, the policy defaults, the HSTS string — is
// arithmetic over configuration with no request in it.
type SecurityHeadersConfig struct {
	Enabled            bool   `default:"true"`
	ContentTypeOptions bool   `default:"true" dependon:".enabled"`
	FrameOptions       string `default:"deny" dependon:".enabled"`
	ReferrerPolicy     string `default:"strict-origin-when-cross-origin" dependon:".enabled"`
	// ContentSecurityPolicy ships with DefaultContentSecurityPolicy rather than
	// empty. Setting it replaces that value entirely; "off" sends no policy.
	ContentSecurityPolicy           string     `default:"script-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'" env:"-" dependon:".enabled" help:"Content-Security-Policy value; off sends none"`
	ContentSecurityPolicyReportOnly string     `env:"-" dependon:".enabled"`
	PermissionsPolicy               string     `env:"-" dependon:".enabled"`
	HSTS                            HSTSConfig `dependon:".enabled"`
}

// DefaultContentSecurityPolicy is the policy a project gets without naming one.
//
// It restricts the four directives a web application can almost always accept,
// and leaves alone the ones it cannot: images, fonts, styles, and connections
// are unrestricted, so an ordinary page keeps working without anyone editing
// configuration.
//
//   - script-src 'self' is the load-bearing one. It refuses inline event
//     handlers, inline <script>, and javascript: URLs, which together are how an
//     HTML-injection sink becomes running code. It also matters more here than
//     in a framework without a browser runtime: the CSRF companion cookie is
//     readable by script on purpose, so script that runs on this origin can mint
//     a valid token. The framework's own runtime is a same-origin module tag and
//     needs nothing else.
//   - object-src 'none' closes <object> and <embed>, which route around
//     script-src on some engines.
//   - base-uri 'self' stops an injected <base href> from re-pointing every
//     relative URL on the page, including the runtime's own.
//   - frame-ancestors 'none' says what X-Frame-Options: DENY already says, in
//     the header that is not deprecated.
//
// A project that loads third-party script names its own policy. That is the
// conversation this default is for: a CSP that shipped empty was one nobody had.
const DefaultContentSecurityPolicy = "script-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'"

// HSTSConfig controls Strict-Transport-Security on verified HTTPS requests.
// The dependon on the HSTS field itself carries the headers switch down over
// this whole block, so nothing here names an absolute key.
type HSTSConfig struct {
	Enabled           bool          `default:"false"`
	MaxAge            time.Duration `default:"0s" dependon:".enabled"`
	IncludeSubdomains bool          `default:"false" dependon:".enabled"`
	Preload           bool          `default:"false" dependon:".enabled"`
}

// DefaultSecurityHeaders returns the classic mode defaults.
func DefaultSecurityHeaders() SecurityHeadersConfig {
	return SecurityHeadersConfig{
		Enabled:               true,
		ContentTypeOptions:    true,
		FrameOptions:          "DENY",
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		ContentSecurityPolicy: DefaultContentSecurityPolicy,
	}
}

// policyOff is the value that sends no policy header, for the two policies whose
// empty value now means the default rather than silence.
const policyOff = "off"

// headerPolicy resolves a configured policy to the value to send, and reports
// whether to send one at all.
//
// Empty means the default and "off" means nothing, which is the inverse of what
// empty used to mean. The swap is deliberate: a policy that is absent by default
// is one a project has to know to ask for, and the projects that most need this
// one are the projects least likely to.
func headerPolicy(configured string) (string, bool) {
	if strings.EqualFold(strings.TrimSpace(configured), policyOff) {
		return "", false
	}
	return configured, configured != ""
}

// Validate rejects response splitting and unsupported fixed-value policies.
func (c SecurityHeadersConfig) Validate() error {
	for name, value := range map[string]string{
		"frame_options": c.FrameOptions, "referrer_policy": c.ReferrerPolicy,
		"content_security_policy":             c.ContentSecurityPolicy,
		"content_security_policy_report_only": c.ContentSecurityPolicyReportOnly,
		"permissions_policy":                  c.PermissionsPolicy,
	} {
		if !validHeaderValue(value) {
			return fmt.Errorf("popcornwave: %s contains an invalid header value", name)
		}
	}
	frame := strings.ToUpper(c.FrameOptions)
	if frame != "" && frame != "OFF" && frame != "DENY" && frame != "SAMEORIGIN" {
		return fmt.Errorf("popcornwave: unsupported frame_options %q", c.FrameOptions)
	}
	if c.ReferrerPolicy != "" {
		switch strings.ToLower(c.ReferrerPolicy) {
		case "no-referrer", "same-origin", "strict-origin", "strict-origin-when-cross-origin":
		default:
			return fmt.Errorf("popcornwave: unsupported referrer_policy %q", c.ReferrerPolicy)
		}
	}
	if c.HSTS.Enabled {
		if c.HSTS.MaxAge <= 0 {
			return fmt.Errorf("popcornwave: HSTS max age must be positive")
		}
		if c.HSTS.Preload && (!c.HSTS.IncludeSubdomains || c.HSTS.MaxAge < 365*24*time.Hour) {
			return fmt.Errorf("popcornwave: HSTS preload requires include_subdomains and a max age of at least one year")
		}
	}
	return nil
}

func validHeaderValue(value string) bool {
	for _, r := range value {
		if (r < 0x20 && r != '\t') || r == 0x7f {
			return false
		}
	}
	return true
}

// ResponseHeader is one name and value to set on a response.
type ResponseHeader struct{ Name, Value string }

// ResolvedSecurityHeaders is a validated configuration reduced to what a
// response actually carries.
type ResolvedSecurityHeaders struct {
	// Always is set on every response. None of these values depends on the
	// request, so they are computed once rather than per request.
	Always []ResponseHeader
	// HSTS is sent only on a request that arrived over HTTPS, and is empty when
	// the feature is off. The condition is about the connection rather than the
	// value, which is why this one is separate rather than in Always.
	HSTS string
}

// ResolveSecurityHeaders validates a configuration and reduces it to the header
// set a response carries.
//
// Both middleware chains call it at construction, so a misconfiguration is an
// error before the port is bound rather than a wrong header per request, and
// the two transports send the same headers because they are computed here
// rather than in either of them.
func ResolveSecurityHeaders(config SecurityHeadersConfig) (ResolvedSecurityHeaders, error) {
	if err := config.Validate(); err != nil {
		return ResolvedSecurityHeaders{}, err
	}
	var resolved ResolvedSecurityHeaders
	add := func(name, value string) {
		if value != "" {
			resolved.Always = append(resolved.Always, ResponseHeader{Name: name, Value: value})
		}
	}
	if config.ContentTypeOptions {
		add("X-Content-Type-Options", "nosniff")
	}
	if frame := strings.ToUpper(config.FrameOptions); frame == "DENY" || frame == "SAMEORIGIN" {
		add("X-Frame-Options", frame)
	}
	add("Referrer-Policy", config.ReferrerPolicy)
	if csp, send := headerPolicy(config.ContentSecurityPolicy); send {
		add("Content-Security-Policy", csp)
	}
	if report, send := headerPolicy(config.ContentSecurityPolicyReportOnly); send {
		add("Content-Security-Policy-Report-Only", report)
	}
	add("Permissions-Policy", config.PermissionsPolicy)
	if config.HSTS.Enabled {
		resolved.HSTS = "max-age=" + strconv.FormatInt(int64(config.HSTS.MaxAge/time.Second), 10)
		if config.HSTS.IncludeSubdomains {
			resolved.HSTS += "; includeSubDomains"
		}
		if config.HSTS.Preload {
			resolved.HSTS += "; preload"
		}
	}
	return resolved, nil
}
