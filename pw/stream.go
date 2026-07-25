package pw

import (
	"errors"
	"net/http"
	"strings"

	tinybind "github.com/shibukawa/tinybind-go"
)

type Stream[T any] struct {
	stream *tinybind.Stream[T]
	err    error
}

func NewStream[T any](w http.ResponseWriter, r *http.Request) *Stream[T] {
	if !supportedStreamAccept(r) {
		err := Problem{Status: http.StatusNotAcceptable, Title: "Not Acceptable", Code: "not_acceptable", Message: "unsupported streaming representation"}
		WriteProblem(w, r, err)
		return &Stream[T]{err: err}
	}
	stream, err := tinybind.NewStream[T](w, r)
	if err != nil {
		WriteProblem(w, r, err)
	}
	return &Stream[T]{stream: stream, err: err}
}

func (s *Stream[T]) Send(value T) error {
	if s == nil {
		return errors.New("popcornwave: nil stream")
	}
	if s.err != nil {
		return s.err
	}
	return s.stream.Write(value)
}

func (s *Stream[T]) Close() error {
	if s == nil || s.stream == nil {
		return nil
	}
	return s.stream.Close()
}

func supportedStreamAccept(r *http.Request) bool {
	if r == nil {
		return true
	}
	accept := strings.TrimSpace(strings.ToLower(r.Header.Get("Accept")))
	if accept == "" || accept == "*/*" {
		return true
	}
	for _, item := range strings.Split(accept, ",") {
		media := strings.TrimSpace(strings.SplitN(item, ";", 2)[0])
		switch media {
		case "*/*", "text/event-stream", "application/x-ndjson", "application/ndjson", "application/json", "application/jsonl":
			return true
		}
	}
	return false
}
