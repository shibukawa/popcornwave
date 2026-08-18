package pw

import (
	"io"
	"net/http"

	"github.com/shibukawa/popcornweb/internal/apidoc"
)

// Supported server.api_doc values. An empty value disables the endpoint.
const (
	APIDocScalar  = apidoc.APIDocScalar
	APIDocSwagger = apidoc.APIDocSwagger
)

// ScalarUI serves a Scalar API reference page for the OpenAPI document at
// specURL. Assets load from a public CDN; nothing is embedded in the binary.
func ScalarUI(specURL string) http.Handler { return apiDocHandler(apidoc.APIDocScalar, specURL) }

// SwaggerUI serves a Swagger UI page for the OpenAPI document at specURL.
// Assets load from a public CDN; nothing is embedded in the binary.
func SwaggerUI(specURL string) http.Handler { return apiDocHandler(apidoc.APIDocSwagger, specURL) }

// apiDocUI returns the page for a configured kind, or nil where the
// configuration names no UI.
func apiDocUI(kind, specURL string) http.Handler {
	if handler := apiDocHandler(kind, specURL); handler != nil {
		return handler
	}
	return nil
}

// apiDocHandler writes the composed page. The composition and the policy are
// the shared package's, so both transports serve the same page under the same
// policy.
func apiDocHandler(kind, specURL string) http.Handler {
	page, ok := apidoc.Build(kind, specURL)
	if !ok {
		return nil
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		if page.CSP != "" {
			for _, name := range apidoc.RelaxedPolicyNames {
				if header.Get(name) != "" {
					header.Set(name, page.CSP)
				}
			}
		}
		header.Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = io.WriteString(w, page.HTML)
		}
	})
}
