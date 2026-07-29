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
	"time"

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
		Logger(requestContext(r)).Log(requestContext(r), LevelError, "problem after response commit", Err(err))
		return
	}
	p := mapProblem(err)
	if p.Status >= 500 {
		Logger(requestContext(r)).Log(requestContext(r), LevelError, "request failed", Err(err))
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
//
// A client that will not run the boundary runtime is the exception, and it
// needs no separate path: the buffered branch already blocks until every
// boundary settles, so classifying the client is enough to hand a crawler the
// finished document instead of the fallbacks it can never replace.
func WriteHTMLChain(w http.ResponseWriter, r *http.Request, wrappers []HTMLWrapper, leaf HTMLFragment) {
	config := Config[HTMLConfig](requestContext(r))
	// The probe runs first because it is the cheapest of the three gates and the
	// only one that can rule streaming out entirely, so a page that could never
	// stream never classifies its client.
	async := htmlbind.HasAwaitBlock(wrappers, leaf)
	bot := false
	if async {
		bot = isBotRequest(r, config)
		// Two branches now produce two byte representations of one URL, so a
		// shared cache must not hand a streamed body to a crawler. Only an
		// await-capable chain pays for this: a page with one representation
		// keeps a response that varies on nothing.
		w.Header().Add("Vary", "User-Agent")
	}
	if async && config.Streaming && !bot {
		streamHTMLChain(w, r, wrappers, leaf, config)
		return
	}
	ctx, cancel := boundedRenderContext(requestContext(r), config, async, bot)
	defer cancel()
	var body bytes.Buffer
	if err := htmlbind.RenderChain(&body, wrappers, leaf, renderOptions(ctx, config, false)...); err != nil {
		// Nothing is committed on this branch, so the same failure the streaming
		// branch can only patch into a 200 still carries its real status here.
		var unrecovered *htmlbind.UnrecoveredError
		if errors.As(err, &unrecovered) {
			Logger(requestContext(r)).Log(requestContext(r), LevelError,
				"await boundary failed with no recover clause", Err(unrecovered.Err))
			writeHTMLProblem(w, r, wrappers, mapProblem(unrecovered.Err))
			return
		}
		WriteProblem(w, r, InternalServerError(err))
		return
	}
	commitHTMLBody(w, r, &body)
}

// WriteHTMLFragment renders one generated template as the whole response, with
// no document shell, no merged head, and no wrapper chain. It answers a partial
// request from an htmx-style swap library, whose target document already exists:
// composing the registered shell around the region would swap a second html,
// head, and body into the live page.
//
// The response is always buffered. The streaming framing is only meaningful
// where the browser parser consumes the response as it arrives, and here the
// swap library holds the body and inserts it, so no marker the framework wrote
// could connect a completion to its placeholder. Blocking on await boundaries
// instead settles them in place and emits no placeholder at all, so a fragment
// carries no boundary id that could duplicate one still pending in the document
// it lands in, and needs no client runtime to be complete.
//
// A fragment carrying head contributions is a programming error rather than a
// silent drop. A component's style block is scoped into the document head, so
// dropping it would swap in an unstyled region with nothing in any log; there is
// no head here to receive it, and inlining the tags would re-emit them on every
// swap with nothing owning or deduplicating them.
func WriteHTMLFragment(w http.ResponseWriter, r *http.Request, fragment HTMLFragment) {
	ctx := requestContext(r)
	config := Config[HTMLConfig](ctx)
	// Contributions fold upward at generation time, so this covers the leaf's own
	// head element and every component it calls statically.
	if head := fragment.Head(); len(head) > 0 {
		WriteProblem(w, r, InternalServerError(fmt.Errorf(
			"popcornwave: HTML fragment declares head contributions a fragment response cannot deliver: %v", head)))
		return
	}
	// Nothing classifies the client here: one branch means one representation, so
	// this response varies on nothing and stays cacheable.
	renderCtx, cancel := boundedRenderContext(ctx, config, fragment.HasAwaitBlock(), false)
	defer cancel()
	var body bytes.Buffer
	if err := htmlbind.Render(&body, fragment, renderOptions(renderCtx, config, false)...); err != nil {
		// Nothing is committed yet, so every failure still carries its real
		// status. It goes out as a problem response rather than as the HTML error
		// page: an error document swapped into a region would replace that region
		// with a whole page, and a swap library already reads the status instead.
		var unrecovered *htmlbind.UnrecoveredError
		if errors.As(err, &unrecovered) {
			Logger(ctx).Log(ctx, LevelError, "await boundary failed with no recover clause", Err(unrecovered.Err))
			WriteProblem(w, r, InternalServerError(unrecovered.Err))
			return
		}
		WriteProblem(w, r, InternalServerError(err))
		return
	}
	commitHTMLBody(w, r, &body)
}

// commitHTMLBody sends a fully rendered body. Content-Length is declared only
// when the bytes reach the client unencoded, because a compressing writer knows
// the length only after it has closed.
func commitHTMLBody(w http.ResponseWriter, r *http.Request, body *bytes.Buffer) {
	ctx := requestContext(r)
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
		Logger(ctx).Log(ctx, LevelError, "HTML response write failed", Err(err))
	}
	if err := closeWriter(); err != nil {
		Logger(ctx).Log(ctx, LevelError, "HTML response close failed", Err(err))
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
	for content, err := range htmlbind.RenderChainAsync(ctx, writer, wrappers, leaf, renderOptions(ctx, config, false)...) {
		if err != nil {
			// A boundary that failed with no recover clause is reported after the
			// initial pass by construction, so the document is already committed
			// and the page can only be repaired from the inside.
			var unrecovered *htmlbind.UnrecoveredError
			if errors.As(err, &unrecovered) {
				logger.Log(ctx, LevelError, "await boundary failed with no recover clause",
					String("boundary", unrecovered.BoundaryID), Err(unrecovered.Err))
				if err := writeDocumentEscalation(writer, mapProblem(unrecovered.Err)); err != nil {
					logger.Log(ctx, LevelError, "HTML error page write failed", Err(err))
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
			logger.Log(ctx, LevelError, "HTML stream failed after commit", Err(err))
			break
		}
		if err := writeBoundaryCompletion(writer, content); err != nil {
			logger.Log(ctx, LevelError, "HTML boundary write failed", Err(err))
			break
		}
		htmlbind.Flush(writer)
	}
	if err := closeWriter(); err != nil {
		logger.Log(ctx, LevelError, "HTML response close failed", Err(err))
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

// boundaryTimeout is the bound one render applies to the work behind its await
// boundaries. A bot request gets its own value because it waits for every
// boundary before a single byte leaves; the browser bound answers a different
// question, namely how long a fallback may sit on screen.
//
// Zero BotAsyncTimeout falls back to the browser bound rather than meaning
// unbounded, so a misread key cannot hold a crawler connection open for the
// whole request deadline.
func boundaryTimeout(config HTMLConfig, bot bool) time.Duration {
	if bot && config.BotAsyncTimeout > 0 {
		return config.BotAsyncTimeout
	}
	return config.AsyncTimeout
}

// boundedRenderContext applies that bound on the buffered branch, where
// htmlbind.WithAsyncTimeout does not reach: the option is read by the async
// coordinator, and the blocking path never builds one. Without this, a chain
// forced onto this branch — by configuration or by a classified bot — would
// wait on its boundaries until the request context ended, which is precisely
// the stall the timeout exists to prevent.
//
// The bound is per render here rather than per boundary. That is the more
// useful shape for this branch anyway: nothing is on the wire yet, so what
// matters is how long the client waits in total, and the work itself already
// runs concurrently because Go started it before the render began.
func boundedRenderContext(ctx context.Context, config HTMLConfig, async, bot bool) (context.Context, context.CancelFunc) {
	timeout := boundaryTimeout(config, bot)
	if !async || timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

// renderOptions builds the htmlbind options for one render. The bound reaches
// the streaming branch as an option and the buffered branch as a context
// deadline, because only one of the two paths reads the option.
func renderOptions(ctx context.Context, config HTMLConfig, bot bool) []htmlbind.Option {
	options := []htmlbind.Option{
		htmlbind.WithContext(ctx),
		htmlbind.WithErrorReporter(func(err error) {
			Logger(ctx).Log(ctx, LevelError, "await boundary failed", Err(err))
		}),
	}
	if timeout := boundaryTimeout(config, bot); timeout > 0 {
		options = append(options, htmlbind.WithAsyncTimeout(timeout))
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
