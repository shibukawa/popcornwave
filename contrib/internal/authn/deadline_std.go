//go:build !tinygo

package authn

import "net/http"

// enforceDeadlines is identity on host Go, whose net/http already cancels a
// request when its context is done.
func enforceDeadlines(client *http.Client) *http.Client { return client }
