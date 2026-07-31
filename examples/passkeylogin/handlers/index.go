package handlers

import "github.com/shibukawa/popcornwave/pw"

var mux = pw.NewServeMux()

func Handlers() *pw.ServeMux { return mux }
