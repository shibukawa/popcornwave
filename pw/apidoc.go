package pw

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Supported server.api_doc values. An empty value disables the endpoint.
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

// ScalarUI serves a Scalar API reference page for the OpenAPI document at
// specURL. Assets load from a public CDN; nothing is embedded in the binary.
//
//	mux.Handle("GET /docs/{$}", pw.ScalarUI("/openapi.json"))
func ScalarUI(specURL string) http.Handler {
	return apiDocPage(scalarUIPage, specURL, scalarAssetURL)
}

// SwaggerUI serves a Swagger UI page for the OpenAPI document at specURL.
// Assets load from a public CDN; nothing is embedded in the binary.
func SwaggerUI(specURL string) http.Handler {
	return apiDocPage(swaggerUIPage, specURL, swaggerUIAssetBase)
}

// apiDocUI returns the handler for a validated server.api_doc value.
func apiDocUI(kind, specURL string) http.Handler {
	switch kind {
	case APIDocScalar:
		return ScalarUI(specURL)
	case APIDocSwagger:
		return SwaggerUI(specURL)
	default:
		return nil
	}
}

func apiDocPage(template, specURL, assetURL string) http.Handler {
	if strings.TrimSpace(specURL) == "" {
		specURL = "/openapi.json"
	}
	page := strings.ReplaceAll(template, "__SPEC_URL__", safeJSString(specURL))
	policy := apiDocCSP(assetOrigin(assetURL))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		relaxAPIDocCSP(header, policy)
		header.Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = io.WriteString(w, page)
		}
	})
}

// relaxAPIDocCSP replaces the application's policy with the one this page needs.
//
// The security headers middleware wraps the operational endpoints, so its policy
// is already on the header while the response is still uncommitted. Rewriting it
// here scopes the CDN and inline allowances to this one page, where widening
// security.headers.content_security_policy would carry them into every response
// the application sends. An application that configures no policy keeps none:
// the header is only replaced where it already exists.
func relaxAPIDocCSP(header http.Header, policy string) {
	if policy == "" {
		return
	}
	for _, name := range []string{"Content-Security-Policy", "Content-Security-Policy-Report-Only"} {
		if header.Get(name) != "" {
			header.Set(name, policy)
		}
	}
}

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
