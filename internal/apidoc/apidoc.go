// Package apidoc composes the documentation page an OpenAPI document is read
// through.
//
// All of it is composition: a template with the spec URL substituted, and the
// content policy that page needs to load its bundle. None of it touches a
// request, which is why it is here rather than in either runtime — the page a
// reader gets should not depend on which transport served it, and the policy
// least of all.
package apidoc

import (
	"encoding/json"
	"net/url"
	"strings"
)

const (
	APIDocScalar  = "scalar"
	APIDocSwagger = "swagger"
)

// Pinned CDN builds. The documents are a few megabytes of JavaScript, so they
// are loaded by the browser instead of being embedded in the binary.
const (
	scalarAssetURL     = "https://cdn.jsdelivr.net/npm/@scalar/api-reference@1.63.0"
	swaggerUIAssetBase = "https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14"
)

// Page is a composed documentation page.
type Page struct {
	// HTML is the document to send.
	HTML string
	// CSP is the policy this page needs, or empty when the pinned asset URL is
	// unusable and no relaxation should happen.
	CSP string
}

// Build composes the page for one UI kind, and reports whether that kind names
// a UI at all.
func Build(kind, specURL string) (Page, bool) {
	var template, assetURL string
	switch kind {
	case APIDocScalar:
		template, assetURL = scalarUIPage, scalarAssetURL
	case APIDocSwagger:
		template, assetURL = swaggerUIPage, swaggerUIAssetBase
	default:
		return Page{}, false
	}
	if strings.TrimSpace(specURL) == "" {
		specURL = "/openapi.json"
	}
	return Page{
		HTML: strings.ReplaceAll(template, "__SPEC_URL__", safeJSString(specURL)),
		CSP:  apiDocCSP(assetOrigin(assetURL)),
	}, true
}

// RelaxedPolicyNames are the two headers a relaxation replaces, in the order a
// caller should walk them.
//
// The replacement is scoped to this one page on purpose. The security header
// frame wraps the documentation endpoint, so the application's policy is
// already on the response while it is still uncommitted; widening the
// configured policy instead would carry the CDN and inline allowances into
// every response the application sends. A policy is only ever replaced where
// one already exists, so an application configuring none keeps none.
var RelaxedPolicyNames = []string{"Content-Security-Policy", "Content-Security-Policy-Report-Only"}

// apiDocCSP is what the documentation page actually loads: the pinned CDN for
// the bundle, plus inline script and style, because the page initialises the UI
// from a <script> element and the UI writes style attributes as it renders.
func apiDocCSP(origin string) string {
	if origin == "" {
		return ""
	}
	return strings.Join([]string{
		"script-src 'self' " + origin + " 'unsafe-inline'",
		"style-src 'self' " + origin + " 'unsafe-inline'",
		"img-src 'self' data:",
		"font-src 'self' " + origin + " data:",
		"connect-src 'self'",
	}, "; ")
}

// assetOrigin reduces a pinned CDN URL to the scheme and host a policy names.
// Deriving it keeps the policy correct when a pin moves to another host.
func assetOrigin(assetURL string) string {
	parsed, err := url.Parse(assetURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

// safeJSString renders a JavaScript string literal. json.Marshal escapes the
// HTML-significant runes, so a configured spec URL cannot terminate the inline
// script element.
func safeJSString(s string) string {
	literal, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(literal)
}

const scalarUIPage = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>API documentation</title>
</head>
<body>
  <div id="app"></div>
  <script src="` + scalarAssetURL + `"></script>
  <script>
    Scalar.createApiReference("#app", {url: __SPEC_URL__});
  </script>
</body>
</html>
`

const swaggerUIPage = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>API documentation</title>
  <link rel="stylesheet" href="` + swaggerUIAssetBase + `/swagger-ui.css"/>
  <style>
    body { margin: 0; background: #fafafa; }
    #swagger-ui { max-width: 100%; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="` + swaggerUIAssetBase + `/swagger-ui-bundle.js"></script>
  <script src="` + swaggerUIAssetBase + `/swagger-ui-standalone-preset.js"></script>
  <script>
    window.onload = function () {
      window.ui = SwaggerUIBundle({
        url: __SPEC_URL__,
        dom_id: "#swagger-ui",
        deepLinking: true,
        presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
        layout: "StandaloneLayout",
        tryItOutEnabled: true
      });
    };
  </script>
</body>
</html>
`
