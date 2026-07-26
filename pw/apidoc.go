package pw

import (
	"encoding/json"
	"io"
	"net/http"
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
func ScalarUI(specURL string) http.Handler { return apiDocPage(scalarUIPage, specURL) }

// SwaggerUI serves a Swagger UI page for the OpenAPI document at specURL.
// Assets load from a public CDN; nothing is embedded in the binary.
func SwaggerUI(specURL string) http.Handler { return apiDocPage(swaggerUIPage, specURL) }

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

func apiDocPage(template, specURL string) http.Handler {
	if strings.TrimSpace(specURL) == "" {
		specURL = "/openapi.json"
	}
	page := strings.ReplaceAll(template, "__SPEC_URL__", safeJSString(specURL))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = io.WriteString(w, page)
		}
	})
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
