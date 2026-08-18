//go:build !fasthttp

package handlers

import "github.com/shibukawa/popcornweb/pw"

var mux = pw.NewServeMux()

func Handlers() *pw.ServeMux { return mux }
