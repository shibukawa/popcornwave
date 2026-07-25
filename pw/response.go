package pw

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	tinybind "github.com/shibukawa/tinybind-go"
	"github.com/shibukawa/tinybind-go/htmlbind"
)

// Problem is the application-facing RFC problem value.
type Problem struct {
	Status  int
	Title   string
	Code    string
	Message string
	Cause   error
}

func (p Problem) Error() string {
	if p.Message != "" {
		return p.Message
	}
	if p.Title != "" {
		return p.Title
	}
	return http.StatusText(p.Status)
}

func (p Problem) Unwrap() error { return p.Cause }

func problem(status int, title string, value any) Problem {
	p := Problem{Status: status, Title: title, Code: strings.ReplaceAll(strings.ToLower(title), " ", "_")}
	switch value := value.(type) {
	case nil:
		p.Message = title
	case Problem:
		if value.Status == 0 {
			value.Status = status
		}
		if value.Title == "" {
			value.Title = title
		}
		return value
	case error:
		p.Message, p.Cause = value.Error(), value
	case string:
		p.Message = value
	default:
		p.Message = fmt.Sprint(value)
	}
	return p
}

func firstValue(values []any) any {
	if len(values) == 0 {
		return nil
	}
	return values[0]
}

func BadRequest(values ...any) Problem {
	return problem(http.StatusBadRequest, "Bad Request", firstValue(values))
}
func NotFound(values ...any) Problem {
	return problem(http.StatusNotFound, "Not Found", firstValue(values))
}
func Forbidden(values ...any) Problem {
	return problem(http.StatusForbidden, "Forbidden", firstValue(values))
}
func InternalServerError(values ...any) Problem {
	p := problem(http.StatusInternalServerError, "Internal Server Error", firstValue(values))
	p.Code = "internal"
	return p
}

func WriteProblem(w http.ResponseWriter, r *http.Request, err error) {
	if responseCommitted(w) {
		Logger(requestContext(r)).ErrorContext(requestContext(r), "problem after response commit", "error", err)
		return
	}
	p := mapProblem(err)
	if p.Status >= 500 {
		Logger(requestContext(r)).ErrorContext(requestContext(r), "request failed", "error", err)
		p.Message = "internal error"
		p.Code = "internal"
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type": "about:blank", "title": p.Title, "status": p.Status,
		"detail": p.Message, "code": p.Code,
	})
}

func responseCommitted(w http.ResponseWriter) bool {
	for w != nil {
		if committed, ok := w.(interface{ Committed() bool }); ok && committed.Committed() {
			return true
		}
		unwrapper, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			return false
		}
		next := unwrapper.Unwrap()
		if next == w {
			return false
		}
		w = next
	}
	return false
}

func mapProblem(err error) Problem {
	if err == nil {
		return InternalServerError(errors.New("nil error"))
	}
	var p Problem
	if errors.As(err, &p) {
		if p.Status == 0 {
			p.Status = http.StatusInternalServerError
		}
		if p.Title == "" {
			p.Title = http.StatusText(p.Status)
		}
		return p
	}
	if mapped, ok := tinybind.AsHTTPError(err); ok {
		message := mapped.Problem.Message
		if message == "" {
			message = mapped.Title
		}
		return Problem{Status: mapped.Status, Title: mapped.Title, Code: mapped.Problem.Code, Message: message, Cause: err}
	}
	return InternalServerError(err)
}

func WriteAPI[T any](w http.ResponseWriter, r *http.Request, value T) {
	if err := tinybind.Write(w, r, value); err != nil {
		WriteProblem(w, r, err)
	}
}

// WriteHTML buffers generated template output before committing the response.
func WriteHTML[P any](w http.ResponseWriter, r *http.Request, template func(io.Writer, P) error, params P) {
	if template == nil {
		WriteProblem(w, r, InternalServerError("nil HTML template"))
		return
	}
	var body bytes.Buffer
	if err := template(&body, params); err != nil {
		WriteProblem(w, r, InternalServerError(err))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer, closeWriter, err := htmlbind.PrepareResponse(w, r)
	if err != nil {
		WriteProblem(w, r, InternalServerError(err))
		return
	}
	if writer == w {
		w.Header().Set("Content-Length", strconv.Itoa(body.Len()))
	}
	if _, err := body.WriteTo(writer); err != nil {
		Logger(requestContext(r)).ErrorContext(requestContext(r), "HTML response write failed", "error", err)
	}
	if err := closeWriter(); err != nil {
		Logger(requestContext(r)).ErrorContext(requestContext(r), "HTML response close failed", "error", err)
	}
}

func requestContext(r *http.Request) context.Context {
	if r == nil {
		return context.Background()
	}
	return r.Context()
}
