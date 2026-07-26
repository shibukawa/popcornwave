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
	"sync"

	"github.com/shibukawa/popcornwave/middlewares"
	tinybind "github.com/shibukawa/tinybind-go"
	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinygodriver/compress/zstd"
)

// Problem is the application-facing RFC problem value.
type Problem struct {
	Status  int
	Title   string
	Code    string
	Message string
	Cause   error
}

// HTMLFragment is a generated template with its parameters already bound.
type HTMLFragment = htmlbind.Fragment

// HTMLWrapper is a generated template wrapper accepted by WriteHTMLChain.
type HTMLWrapper = htmlbind.Wrapper

var documentState = struct {
	sync.RWMutex
	wrapper *HTMLWrapper
}{}

// RegisterHTMLDocument installs the generated application document shell.
// It is intended for generated templates/document_pw_gen.go code.
func RegisterHTMLDocument(wrapper HTMLWrapper) {
	documentState.Lock()
	defer documentState.Unlock()
	if documentState.wrapper != nil {
		panic("popcornwave: HTML document is already registered")
	}
	documentState.wrapper = &wrapper
}

func registeredHTMLDocument() []HTMLWrapper {
	documentState.RLock()
	defer documentState.RUnlock()
	if documentState.wrapper == nil {
		return nil
	}
	return []HTMLWrapper{*documentState.wrapper}
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
func Unauthorized(values ...any) Problem {
	return problem(http.StatusUnauthorized, "Unauthorized", firstValue(values))
}
func Forbidden(values ...any) Problem {
	return problem(http.StatusForbidden, "Forbidden", firstValue(values))
}
func ServiceUnavailable(values ...any) Problem {
	return problem(http.StatusServiceUnavailable, "Service Unavailable", firstValue(values))
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

func responseCommitted(w http.ResponseWriter) bool { return middlewares.Committed(w) }

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

// WriteHTML renders one generated HTML fragment.
func WriteHTML(w http.ResponseWriter, r *http.Request, leaf HTMLFragment) {
	WriteHTMLChain(w, r, registeredHTMLDocument(), leaf)
}

// WriteHTMLChain renders generated wrappers around one leaf without committing
// the response until TinyBind has validated and rendered the complete chain.
func WriteHTMLChain(w http.ResponseWriter, r *http.Request, wrappers []HTMLWrapper, leaf HTMLFragment) {
	var body bytes.Buffer
	if err := htmlbind.RenderChain(&body, wrappers, leaf); err != nil {
		WriteProblem(w, r, InternalServerError(err))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer, closeWriter, err := prepareHTMLResponse(w, r)
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

func prepareHTMLResponse(w http.ResponseWriter, r *http.Request) (io.Writer, func() error, error) {
	if !Config[MiddlewareConfig](requestContext(r)).Compression {
		return w, func() error { return nil }, nil
	}
	addVaryHeader(w.Header(), "Accept-Encoding")
	if r == nil || !acceptsZstdEncoding(r.Header.Values("Accept-Encoding")) || w.Header().Get("Content-Encoding") != "" {
		return w, func() error { return nil }, nil
	}
	w.Header().Set("Content-Encoding", zstd.ContentEncoding)
	w.Header().Del("Content-Length")
	encoder, err := zstd.NewWriter(w, zstd.WithETag(false))
	if err != nil {
		return w, func() error { return nil }, err
	}
	return encoder, encoder.Close, nil
}

func acceptsZstdEncoding(values []string) bool {
	for _, value := range values {
		for _, entry := range strings.Split(value, ",") {
			parts := strings.Split(entry, ";")
			if !strings.EqualFold(strings.TrimSpace(parts[0]), zstd.ContentEncoding) {
				continue
			}
			quality := 1.0
			for _, parameter := range parts[1:] {
				name, raw, ok := strings.Cut(parameter, "=")
				if !ok || !strings.EqualFold(strings.TrimSpace(name), "q") {
					continue
				}
				parsed, parseErr := strconv.ParseFloat(strings.TrimSpace(raw), 64)
				if parseErr != nil || parsed < 0 || parsed > 1 {
					quality = 0
				} else {
					quality = parsed
				}
			}
			if quality > 0 {
				return true
			}
		}
	}
	return false
}

func addVaryHeader(header http.Header, value string) {
	for _, line := range header.Values("Vary") {
		for _, existing := range strings.Split(line, ",") {
			existing = strings.TrimSpace(existing)
			if existing == "*" || strings.EqualFold(existing, value) {
				return
			}
		}
	}
	header.Add("Vary", value)
}

func requestContext(r *http.Request) context.Context {
	if r == nil {
		return context.Background()
	}
	return r.Context()
}
