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
	Fields  []FieldError
	Cause   error
}

// FieldError describes a single field-level validation failure.
type FieldError = tinybind.FieldError

// Field builds a field-level validation error for Validation.
func Field(field, location, message string) FieldError {
	return tinybind.Field(field, location, message)
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
func Unauthorized(values ...any) Problem {
	return problem(http.StatusUnauthorized, "Unauthorized", firstValue(values))
}
func Forbidden(values ...any) Problem {
	return problem(http.StatusForbidden, "Forbidden", firstValue(values))
}
func NotFound(values ...any) Problem {
	return problem(http.StatusNotFound, "Not Found", firstValue(values))
}
func Conflict(values ...any) Problem {
	return problem(http.StatusConflict, "Conflict", firstValue(values))
}
func PayloadTooLarge(values ...any) Problem {
	return problem(http.StatusRequestEntityTooLarge, "Payload Too Large", firstValue(values))
}
func ServiceUnavailable(values ...any) Problem {
	return problem(http.StatusServiceUnavailable, "Service Unavailable", firstValue(values))
}
func InternalServerError(values ...any) Problem {
	p := problem(http.StatusInternalServerError, "Internal Server Error", firstValue(values))
	p.Code = "internal"
	return p
}

// Validation reports a 400 response carrying every detected field failure.
func Validation(fields ...FieldError) Problem {
	p := problem(http.StatusBadRequest, "Validation failed", nil)
	p.Fields = append([]FieldError(nil), fields...)
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
		p.Fields = nil
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	payload := map[string]any{
		"type": "about:blank", "title": p.Title, "status": p.Status,
		"detail": p.Message, "code": p.Code,
	}
	if len(p.Fields) > 0 {
		fields := make([]map[string]string, 0, len(p.Fields))
		for _, field := range p.Fields {
			fields = append(fields, map[string]string{
				"field": field.Field, "location": field.Location, "message": field.Message,
			})
		}
		payload["errors"] = fields
	}
	_ = json.NewEncoder(w).Encode(payload)
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
		return Problem{
			Status: mapped.Status, Title: mapped.Title, Code: mapped.Problem.Code,
			Message: message, Fields: append([]FieldError(nil), mapped.Fields...), Cause: err,
		}
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
//
// A chain that can open an await boundary streams instead: the shell and every
// fallback commit first, and each boundary is written as it settles. Nothing
// about the handler changes, because whether a response streams is a property
// of the templates it composed rather than a decision the handler makes.
func WriteHTMLChain(w http.ResponseWriter, r *http.Request, wrappers []HTMLWrapper, leaf HTMLFragment) {
	config := Config[HTMLConfig](requestContext(r))
	if config.Streaming && htmlbind.HasAwaitBlock(wrappers, leaf) {
		streamHTMLChain(w, r, wrappers, leaf, config)
		return
	}
	var body bytes.Buffer
	if err := htmlbind.RenderChain(&body, wrappers, leaf, renderOptions(r, config)...); err != nil {
		// Nothing is committed on this branch, so the same failure the streaming
		// branch can only patch into a 200 still carries its real status here.
		var unrecovered *htmlbind.UnrecoveredError
		if errors.As(err, &unrecovered) {
			Logger(requestContext(r)).ErrorContext(requestContext(r),
				"await boundary failed with no recover clause", "error", unrecovered.Err)
			writeHTMLProblem(w, r, wrappers, mapProblem(unrecovered.Err))
			return
		}
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

// streamHTMLChain writes the document as htmlbind produces it. The first error
// arrives before anything is written, because chain assembly and the check for
// unset async values both run before the initial pass, so that failure can
// still become a problem response.
func streamHTMLChain(w http.ResponseWriter, r *http.Request, wrappers []HTMLWrapper, leaf HTMLFragment, config HTMLConfig) {
	ctx := requestContext(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer, closeWriter, err := prepareHTMLResponse(w, r)
	if err != nil {
		WriteProblem(w, r, InternalServerError(err))
		return
	}
	logger := Logger(ctx)
	for content, err := range htmlbind.RenderChainAsync(ctx, writer, wrappers, leaf, renderOptions(r, config)...) {
		if err != nil {
			// A boundary that failed with no recover clause is reported after the
			// initial pass by construction, so the document is already committed
			// and the page can only be repaired from the inside.
			var unrecovered *htmlbind.UnrecoveredError
			if errors.As(err, &unrecovered) {
				logger.ErrorContext(ctx, "await boundary failed with no recover clause",
					"boundary", unrecovered.BoundaryID, "error", unrecovered.Err)
				if err := writeDocumentEscalation(writer, mapProblem(unrecovered.Err)); err != nil {
					logger.ErrorContext(ctx, "HTML error page write failed", "error", err)
				}
				htmlbind.Flush(writer)
				break
			}
			if !responseCommitted(w) {
				// Nothing reached the client yet, so this response can still be
				// replaced. The encoding was chosen for a document that will not
				// be written now, and the problem body is not encoded, so the
				// header has to go with it. Any partial document still sitting in
				// the encoder is dropped rather than closed, which is the whole
				// reason it is safe to answer at all.
				w.Header().Del("Content-Encoding")
				WriteProblem(w, r, InternalServerError(err))
				return
			}
			// The status is already on the wire, so this is for the operator
			// rather than for the client: every committed fallback stays.
			logger.ErrorContext(ctx, "HTML stream failed after commit", "error", err)
			break
		}
		if err := writeBoundaryCompletion(writer, content); err != nil {
			logger.ErrorContext(ctx, "HTML boundary write failed", "error", err)
			break
		}
		htmlbind.Flush(writer)
	}
	if err := closeWriter(); err != nil {
		logger.ErrorContext(ctx, "HTML response close failed", "error", err)
	}
}

// writeBoundaryCompletion frames one settled boundary for the browser runtime
// in pw.RuntimeScriptURL. htmlbind yields the bare fragment and the id of the
// placeholder it belongs to; the framing and the script that acts on it are one
// design and both live here.
//
// The trailing marker is what makes the swap safe, and boundaryRuntimeScript
// explains why it cannot be replaced by reacting to the template itself.
func writeBoundaryCompletion(w io.Writer, content htmlbind.Content) error {
	if _, err := io.WriteString(w, `<template data-tb-boundary="`+content.BoundaryID+`">`); err != nil {
		return err
	}
	if _, err := w.Write(content.HTML); err != nil {
		return err
	}
	_, err := io.WriteString(w, `</template><tb-apply for="`+content.BoundaryID+`"></tb-apply>`)
	return err
}

func renderOptions(r *http.Request, config HTMLConfig) []htmlbind.Option {
	ctx := requestContext(r)
	options := []htmlbind.Option{
		htmlbind.WithContext(ctx),
		htmlbind.WithErrorReporter(func(err error) {
			Logger(ctx).ErrorContext(ctx, "await boundary failed", "error", err)
		}),
	}
	if config.AsyncTimeout > 0 {
		options = append(options, htmlbind.WithAsyncTimeout(config.AsyncTimeout))
	}
	if config.AsyncConcurrency > 0 {
		options = append(options, htmlbind.WithConcurrencyLimit(config.AsyncConcurrency))
	}
	return options
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
	return flushingEncoder{encoder: encoder, downstream: w}, encoder.Close, nil
}

// flushingEncoder chains one flush through both layers. zstd deliberately does
// not flush its destination, and a completion sitting in either the encoder or
// the server's buffer defeats the point of having sent it early.
//
// Flushing per boundary rather than per write is what keeps the ratio
// reasonable: each flush ends a block, and a block ended early compresses worse.
type flushingEncoder struct {
	encoder    *zstd.Writer
	downstream http.ResponseWriter
}

func (f flushingEncoder) Write(p []byte) (int, error) { return f.encoder.Write(p) }

func (f flushingEncoder) Flush() {
	if err := f.encoder.Flush(); err != nil {
		return
	}
	if flusher, ok := f.downstream.(http.Flusher); ok {
		flusher.Flush()
	}
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
