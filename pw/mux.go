// Package pw is the stable application-facing Popcorn Wave API.
package pw

import "github.com/shibukawa/tinygodriver/httpmux"

type ServeMux = httpmux.ServeMux

func NewServeMux() *ServeMux { return httpmux.NewServeMux() }
