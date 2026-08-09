package pw

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shibukawa/popcornwave/middlewares"
	"github.com/shibukawa/popcornwave/pwruntime"
	tinybind "github.com/shibukawa/tinybind-go"
	"github.com/shibukawa/tinybind-go/htmlbind"
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

// HTMLOption tunes one render. The framework supplies the options every
// response needs from HTMLConfig, so a caller passing one is extending that set
// rather than replacing it.
type HTMLOption = htmlbind.Option

// documentState holds the registered document shell as a one-element chain.
// Registration happens once at init, so per-request reads take no lock, and
// the cached slice has no spare capacity: any append reallocates rather than
// reaching a slice another request is rendering through.
var documentState atomic.Pointer[[]HTMLWrapper]

// RegisterHTMLDocument installs the generated application document shell.
// It is intended for generated templates/document_pw_gen.go code.
func RegisterHTMLDocument(wrapper HTMLWrapper) {
	chain := []HTMLWrapper{wrapper}
	if !documentState.CompareAndSwap(nil, &chain) {
		panic("popcornwave: HTML document is already registered")
	}
}

func registeredHTMLDocument() []HTMLWrapper {
	chain := documentState.Load()
	if chain == nil {
		return nil
	}
	return *chain
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
	}
	p = sanitizedProblem(p)
	// One handler answers a browser form post and an API client on the same
	// route. Which representation this failure takes is the client's to say, so
	// it is read from Accept rather than branched on by the caller.
	if acceptsHTML(r) && registeredHTMLErrorPage() != nil {
		writeHTMLProblem(w, r, registeredHTMLDocument(), p)
		return
	}
	writeProblemJSON(w, r, p)
}

// sanitizedProblem drops what a 5xx must never carry out of the process.
//
// It sits here rather than in WriteProblem because both representations are
// written from more than one place: a boundary that failed with no recover
// clause reaches the HTML writer directly, and every path that can answer with
// a server error has to lose the cause on the way out. Applying it twice
// changes nothing, which is what makes it safe to put at each writer.
func sanitizedProblem(p Problem) Problem {
	if p.Status < 500 {
		return p
	}
	p.Message = "internal error"
	p.Code = "internal"
	p.Fields = nil
	return p
}

// writeProblemJSON is the API branch, and the fallback for every way the HTML
// branch can decline: no renderer, no fragment, or a render that failed.
func writeProblemJSON(w http.ResponseWriter, r *http.Request, p Problem) {
	p = sanitizedProblem(p)
	addVaryHeader(w.Header(), "Accept")
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	// Built by hand like the root package's writeProblem: the document is flat
	// and known, and this path must not fail.
	var b strings.Builder
	b.WriteString(`{"type":"about:blank","title":`)
	b.WriteString(strconv.Quote(p.Title))
	b.WriteString(`,"status":`)
	b.WriteString(strconv.Itoa(p.Status))
	b.WriteString(`,"detail":`)
	b.WriteString(strconv.Quote(p.Message))
	b.WriteString(`,"code":`)
	b.WriteString(strconv.Quote(p.Code))
	if len(p.Fields) > 0 {
		b.WriteString(`,"errors":[`)
		for i, field := range p.Fields {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(`{"field":`)
			b.WriteString(strconv.Quote(field.Field))
			b.WriteString(`,"location":`)
			b.WriteString(strconv.Quote(field.Location))
			b.WriteString(`,"message":`)
			b.WriteString(strconv.Quote(field.Message))
			b.WriteByte('}')
		}
		b.WriteByte(']')
	}
	b.WriteString("}\n")
	_, _ = io.WriteString(w, b.String())
}

func responseCommitted(w http.ResponseWriter) bool { return middlewares.Committed(w) }

// htmlBodyPool recycles the page-sized buffers of the buffered render paths.
// An outlier beyond the cap is dropped rather than kept alive for the life of
// the pool.
var htmlBodyPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

const maxPooledBodyBytes = 1 << 20

func getHTMLBody() *bytes.Buffer { return htmlBodyPool.Get().(*bytes.Buffer) }

func putHTMLBody(body *bytes.Buffer) {
	if body.Cap() > maxPooledBodyBytes {
		return
	}
	body.Reset()
	htmlBodyPool.Put(body)
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

// WriteStatus is WriteAPI with the success status made explicit: 201 for a
// creation, 202 for an accepted job, 204 for no content — which writes no
// body. The status must be a literal or a named constant at the call site,
// because the generated OpenAPI document lists one response per static status
// the handler calls this with, and a status computed at runtime is one the
// scanner cannot see.
//
// A 204 writes no body, which the library entry point already implements.
//
// Unlike WriteAPI, this cannot answer a failure with a problem document. The
// status and the content type are written before the value is encoded, so
// every error it can return arrives after the response committed — a 204
// writes no body and cannot fail at all — and a problem written over it would
// leave a 2xx carrying an error document. It is logged instead.
//
// What reaches here is a type whose encoder generation never emitted, which
// means the call site never reached pw generate. That is a build mistake, and
// the log names it as one.
func WriteStatus[T any](w http.ResponseWriter, r *http.Request, status int, value T) {
	if err := tinybind.WriteStatus(w, r, status, value); err != nil {
		ctx := r.Context()
		Logger(ctx).Log(ctx, LevelError, "write status failed after the response committed",
			Int("status", status), Err(err))
	}
}

// WriteHTML renders one generated HTML fragment.
func WriteHTML(w http.ResponseWriter, r *http.Request, leaf HTMLFragment) {
	WriteHTMLChain(w, r, registeredHTMLDocument(), leaf)
}

// WriteHTMLPage renders a page inside its own wrapper chain and the registered
// document shell, with the document outermost. It is intended for generated
// page tree code, which knows the ancestor layouts of a route but must not name
// the document: that one stays the framework's, exactly as it is for a handler
// calling WriteHTML.
func WriteHTMLPage(w http.ResponseWriter, r *http.Request, wrappers []HTMLWrapper, leaf HTMLFragment, options ...HTMLOption) {
	// registeredHTMLDocument allocates per call, so appending to it cannot reach
	// a slice another request is rendering through.
	WriteHTMLChain(w, r, append(registeredHTMLDocument(), wrappers...), leaf, options...)
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
func WriteHTMLChain(w http.ResponseWriter, r *http.Request, wrappers []HTMLWrapper, leaf HTMLFragment, options ...HTMLOption) {
	config := Config[HTMLConfig](requestContext(r))
	// Every unsafe form in this chain carries the session's token, and the
	// document path is where that is supplied: WriteHTMLFragment renders no
	// document and shares only the option builder below, so it is untouched.
	//
	// The framework's options go first, so a caller passing its own still wins.
	token := csrfRenderToken(requestContext(r))
	options = append(documentRenderOptions(config, token), options...)
	// The probes are properties of the composed chain, so they run once here
	// and every branch below reads the same two answers. async is the cheapest
	// of the three streaming gates and the only one that can rule streaming out
	// entirely; live answers a different question, whether this screen keeps
	// changing once the document is complete, and never changes which branch
	// runs, because a live block implies an await block.
	async := htmlbind.HasAwaitBlock(wrappers, leaf)
	live := htmlbind.HasLiveBlock(wrappers, leaf)
	// A page that can update answers three ways from one URL, and which one is
	// decided before anything is written. An unrecognized mode resolves to the
	// document, so a crawler, curl, and a browser without the runtime are
	// unaffected by any of this.
	if config.Update.Enabled {
		// A redraw is tested first because it answers with one component's
		// subtree and never touches this chain, so there is nothing about the
		// page left to decide once it has been recognized.
		if serveRegisteredRedraw(w, r, config) {
			return
		}
		if serveUpdate(w, r, wrappers, leaf, config, options, async, live) {
			return
		}
	}
	if liveModeRequested(r) {
		// The handler, the layouts, and the binding that produced this chain have
		// already run, which is what makes a reconnect need no continuation: the
		// reconstruction path is the render path.
		serveLive(w, r, wrappers, leaf, config, options...)
		return
	}
	if live && liveEnabled(config) {
		// One URL now has a document representation and a delivery one. The
		// delivery stream is no-store, so this exists to stop a cache from
		// answering a live request with the stored document.
		addVaryHeader(w.Header(), ResponseModeHeader)
	}
	bot := false
	if async {
		bot = isBotRequest(r, config)
		// Two branches now produce two byte representations of one URL, so a
		// shared cache must not hand a streamed body to a crawler. Only an
		// await-capable chain pays for this: a page with one representation
		// keeps a response that varies on nothing.
		w.Header().Add("Vary", "User-Agent")
	}
	// A browser with scripting disabled sends an ordinary User-Agent, so the
	// classification above says browser and it keeps every fallback. Asking it
	// is only worth doing where it would otherwise be wrong, which is exactly
	// the branch below.
	scriptless := false
	if async && config.Streaming && !bot && config.ScriptlessDetection {
		// A third representation of this URL, selected by the marker cookie
		// rather than by the header above.
		w.Header().Add("Vary", "Cookie")
		buffered, handled := resolveScriptless(w, r)
		switch {
		case handled:
			return
		case buffered:
			scriptless = true
		case scriptlessSafeMethod(r):
			options = append(options, htmlbind.WithHead(scriptlessProbeHead(r)))
		}
	}
	if async && config.Streaming && !bot && !scriptless {
		streamHTMLChain(w, r, wrappers, leaf, config, live, options...)
		return
	}
	// The whole render is the initial build on this branch, so it opens no child
	// span: every await boundary settles in place before the first byte, and
	// there is nothing after the pass for a second span to separate.
	ctx, render := startRenderTrace(requestContext(r), renderModeBuffered,
		chainRenderAttributes(wrappers, async, live, bot)...)
	defer render.end()
	// A scriptless client waits for every boundary before any byte, which is the
	// same shape a classified bot waits in, so it takes the same longer bound
	// rather than the streaming one it will never benefit from.
	ctx, cancel := boundedRenderContext(ctx, config, async, bot || scriptless)
	defer cancel()
	body := getHTMLBody()
	defer putHTMLBody(body)
	if err := htmlbind.RenderChain(body, wrappers, leaf, renderOptions(ctx, config, false, options)...); err != nil {
		render.failed(err)
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
	render.wrote(body.Len())
	commitHTMLBody(w, r, body)
}

// chainRenderAttributes describes the shape of one composed chain, which is
// what decides the branch this response took.
//
// Every value is a property of the templates rather than of the request, so
// none of it can carry an instance key, a component input, or anything a user
// supplied, which is what requirement:modern-observability asks of a dimension.
func chainRenderAttributes(wrappers []HTMLWrapper, async, live, bot bool) []Attribute {
	return []Attribute{
		Int("pw.render.layers", renderLayers(wrappers)),
		Bool("pw.render.async", async),
		Bool("pw.render.live", live),
		Bool("pw.render.bot", bot),
	}
}

// documentRenderOptions is everything the framework contributes to a document
// render: the CSRF token every unsafe form carries, the boundary prefix, and
// the head nodes that load the client runtime.
//
// It is deliberately not part of the option builder the fragment path shares.
// A fragment response renders no document, so it has no head to merge into and
// decision:fragment-head-rejection refuses one that tries.
func documentRenderOptions(config HTMLConfig, csrfToken string) []HTMLOption {
	options := make([]HTMLOption, 0, 3)
	if csrfToken != "" {
		options = append(options, htmlbind.WithCSRFToken(csrfToken))
	}
	if config.Update.Enabled {
		// One prefix names the generated attributes, the placeholder element,
		// and the boundary ids, so a document does not hold two spellings.
		options = append(options, htmlbind.WithBoundaryPrefix(UpdateAttributePrefix))
		if nodes := updateHeadNodes(config, csrfToken); len(nodes) > 0 {
			options = append(options, htmlbind.WithHead(nodes...))
		}
	}
	return options
}

// csrfRenderToken derives the token every unsafe form of this render carries,
// when the request has a secret to take one from.
//
// A request without one yields nothing rather than htmlbind.WithoutCSRFToken:
// that option renders an unsafe form with an empty token, which is right for a
// mail body or a golden test and wrong for a response, where it would put an
// unprotected form on screen and say nothing. Yielding nothing fails the render
// instead, which is the outcome policy:csrf-protection asks for.
func csrfRenderToken(ctx context.Context) string {
	secret, ok := pwruntime.CSRFSecret(ctx)
	if !ok {
		return ""
	}
	token, err := pwruntime.CSRFToken(secret, nil)
	if err != nil {
		return ""
	}
	return token
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
	async := fragment.HasAwaitBlock()
	traceCtx, render := startRenderTrace(ctx, renderModeFragment,
		Int("pw.render.layers", 1), Bool("pw.render.async", async),
		Bool("pw.render.live", false), Bool("pw.render.bot", false))
	defer render.end()
	renderCtx, cancel := boundedRenderContext(traceCtx, config, async, false)
	defer cancel()
	body := getHTMLBody()
	defer putHTMLBody(body)
	if err := htmlbind.Render(body, fragment, renderOptions(renderCtx, config, false, nil)...); err != nil {
		render.failed(err)
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
	render.wrote(body.Len())
	commitHTMLBody(w, r, body)
}

// commitHTMLBody sends a fully rendered body. Content-Length is declared only
// when the bytes reach the client unencoded, because a compressing writer knows
// the length only after it has closed.
func commitHTMLBody(w http.ResponseWriter, r *http.Request, body *bytes.Buffer) {
	ctx := requestContext(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer, closeWriter, _, err := prepareHTMLResponse(w, r)
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
func streamHTMLChain(w http.ResponseWriter, r *http.Request, wrappers []HTMLWrapper, leaf HTMLFragment, config HTMLConfig, live bool, options ...HTMLOption) {
	ctx, render := startRenderTrace(requestContext(r), renderModeStream,
		chainRenderAttributes(wrappers, true, live, false)...)
	defer render.end()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer, closeWriter, abortWriter, err := prepareHTMLResponse(w, r)
	if err != nil {
		render.failed(err)
		WriteProblem(w, r, InternalServerError(err))
		return
	}
	// The wrapper counts the response and turns htmlbind's own flush, the one
	// that ends the initial pass, into the end of the initial build span.
	writer = render.writer(writer)
	render.initialBuild()
	logger := Logger(ctx)
	failed := false
	for content, err := range htmlbind.RenderChainAsync(ctx, writer, wrappers, leaf, renderOptions(ctx, config, false, options)...) {
		if err != nil {
			render.failed(err)
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
				failed = true
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
				abortWriter()
				WriteProblem(w, r, InternalServerError(err))
				return
			}
			// The status is already on the wire, so this is for the operator
			// rather than for the client: every committed fallback stays.
			logger.Log(ctx, LevelError, "HTML stream failed after commit", Err(err))
			failed = true
			break
		}
		render.boundarySettled(content.BoundaryID, len(content.HTML))
		if err := writeBoundaryCompletion(writer, content); err != nil {
			render.failed(err)
			logger.Log(ctx, LevelError, "HTML boundary write failed", Err(err))
			failed = true
			break
		}
		htmlbind.Flush(writer)
	}
	if err := writeStreamEnd(writer, streamEndState(config, live, failed)); err != nil {
		logger.Log(ctx, LevelError, "HTML stream end write failed", Err(err))
	}
	htmlbind.Flush(writer)
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

// writeStreamEnd closes a streamed document with an inert marker naming what,
// if anything, follows it.
//
// Completion cannot be inferred from the transport here. A chunked document cut
// off mid-stream is end of file to the parser, which renders what arrived and
// fires DOMContentLoaded and load with nothing surfaced to the page, and the
// response carries no Content-Length to compare against. So the last bytes say
// it explicitly, and a client that finished parsing without seeing them knows
// it holds a truncated page.
//
// The state is also what keeps a screen that will never change again from
// paying for a live request, which costs a whole page execution to answer with
// nothing.
func writeStreamEnd(w io.Writer, state string) error {
	marker := `<tb-stream-end state="` + state + `"`
	if version := renderVersion(); version != "" {
		marker += ` version="` + htmlbind.Escape(version) + `"`
	}
	_, err := io.WriteString(w, marker+`></tb-stream-end>`)
	return err
}

func streamEndState(config HTMLConfig, live, failed bool) string {
	switch {
	case failed:
		// The committed fallbacks this response left behind are not going to be
		// replaced by it, and the client is told so rather than left waiting.
		return "failed"
	case liveEnabled(config) && live:
		return "live"
	default:
		return "final"
	}
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
func renderOptions(ctx context.Context, config HTMLConfig, bot bool, extra []HTMLOption) []htmlbind.Option {
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
	// Caller options come last so a later one wins, which is what makes them an
	// extension of the configured set rather than a competing source of truth.
	return append(options, extra...)
}

func prepareHTMLResponse(w http.ResponseWriter, r *http.Request) (io.Writer, func() error, func(), error) {
	if !zstdResponseSupported || !Config[MiddlewareConfig](requestContext(r)).Compression {
		return w, func() error { return nil }, func() {}, nil
	}
	addVaryHeader(w.Header(), "Accept-Encoding")
	if r == nil || !acceptsZstdEncoding(r.Header.Values("Accept-Encoding")) || w.Header().Get("Content-Encoding") != "" {
		return w, func() error { return nil }, func() {}, nil
	}
	w.Header().Set("Content-Encoding", zstdContentEncoding)
	w.Header().Del("Content-Length")
	encoder, err := newResponseZstdEncoder(w)
	if err != nil {
		return w, func() error { return nil }, func() {}, err
	}
	return flushingEncoder{encoder: encoder, downstream: w}, encoder.Close, encoder.Abort, nil
}

// flushingEncoder chains one flush through both layers. zstd deliberately does
// not flush its destination, and a completion sitting in either the encoder or
// the server's buffer defeats the point of having sent it early.
//
// Flushing per boundary rather than per write is what keeps the ratio
// reasonable: each flush ends a block, and a block ended early compresses worse.
type flushingEncoder struct {
	encoder    responseZstdEncoder
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
		// Cut loops rather than Split, so a header line parses without
		// allocating.
		for entry := range splitSeq(value, ',') {
			coding, parameters, _ := strings.Cut(entry, ";")
			if !strings.EqualFold(strings.TrimSpace(coding), zstdContentEncoding) {
				continue
			}
			quality := 1.0
			for parameter := range splitSeq(parameters, ';') {
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

// splitSeq yields the separator-delimited pieces of value without allocating.
// Unlike strings.SplitSeq it never yields anything for an empty value, which
// is what a header parse wants.
func splitSeq(value string, separator byte) func(func(string) bool) {
	return func(yield func(string) bool) {
		for value != "" {
			var piece string
			if index := strings.IndexByte(value, separator); index >= 0 {
				piece, value = value[:index], value[index+1:]
			} else {
				piece, value = value, ""
			}
			if !yield(piece) {
				return
			}
		}
	}
}

func addVaryHeader(header http.Header, value string) {
	for _, line := range header.Values("Vary") {
		for existing := range splitSeq(line, ',') {
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
