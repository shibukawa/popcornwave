package pw

import (
	"net/http"

	tinybind "github.com/shibukawa/tinybind-go"
)

func Parse[T any](r *http.Request) (T, error) { return tinybind.Bind[T](r) }
