package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/shibukawa/popcornwave/pw"
	httpbind "github.com/shibukawa/tinybind-go"
)

func init() {
	mux.HandleFunc("GET /{$}", home)
	mux.HandleFunc("GET /tasks", listTasks)
	mux.HandleFunc("POST /tasks", createTask)
	mux.HandleFunc("DELETE /tasks/{id}", removeTask)
	mux.HandleFunc("GET /tasks/summary", taskSummary)
	mux.HandleFunc("GET /tasks/broken", brokenFragment)
}

// home is the only route that answers with a document. It composes the same
// TaskPanel and TaskList components the partial routes below render alone, so
// the first paint and every later swap come from one definition of the markup.
func home(w http.ResponseWriter, r *http.Request) {
	matched := tasks.list("")
	pw.WriteHTML(w, r, Home(HomeParams{
		Form:       FormState{Priority: "normal"},
		Tasks:      matched,
		EmptyLabel: emptyLabel("", len(matched)),
		Query:      "",
	}))
}

type listInput struct {
	// input reads the query string and falls back to the body, which is what
	// makes this handler indifferent to where htmx put the value: a GET carries
	// it in the URL, and a DELETE does so only depending on the client's
	// configuration.
	Query string `input:"q" check:"maxlen=40"`
}

// listTasks answers the filter box. The response is the list region and nothing
// else, so htmx replaces #task-list and leaves the rest of the page — including
// the text being typed into the filter — untouched.
func listTasks(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[listInput](r)
	if err != nil {
		pw.WriteProblem(w, r, pw.BadRequest(err))
		return
	}
	matched := tasks.list(input.Query)
	pw.WriteHTMLFragment(w, r, TaskList(TaskListParams{
		Tasks:      matched,
		EmptyLabel: emptyLabel(input.Query, len(matched)),
	}))
}

type createInput struct {
	Title    string `payload:"title" check:"required,maxlen=60"`
	Owner    string `payload:"owner" check:"required,maxlen=24"`
	Priority string `payload:"priority" enum:"low,normal,high" default:"normal"`
	Query    string `input:"q" check:"maxlen=40"`
}

// createTask is the write, and the one place where the status contract of a
// fragment response has to be thought about.
//
// A failed check is an error, but a typo in a form is expected traffic rather
// than a malformed request, and htmx does not swap a non-2xx response: answering
// with the problem document pw.WriteProblem builds would leave the page showing
// nothing at all about why nothing happened. The checks stay declared on the
// struct; the handler renders their field errors back into the form it re-sends,
// with what the operator typed still in the fields.
func createTask(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[createInput](r)
	if err != nil {
		fields, ok := validationFields(err)
		if !ok {
			// Not a field-level failure — an unreadable body, or one too large.
			// Nothing on the page can be usefully re-rendered for that.
			pw.WriteProblem(w, r, pw.BadRequest(err))
			return
		}
		// pw.Parse answers with the zero value when a check fails, so the text
		// to put back in the fields comes from the request itself. htmx submits
		// this form url-encoded, and binding has already parsed that body, so
		// reading it again costs nothing.
		form := FormState{
			Title:    r.PostFormValue("title"),
			Owner:    r.PostFormValue("owner"),
			Priority: knownPriority(r.PostFormValue("priority")),
		}
		applyFieldErrors(&form, fields)
		writePanel(w, r, form, r.FormValue("q"), "")
		return
	}
	added := tasks.add(input.Title, input.Owner, input.Priority)
	writePanel(w, r, FormState{Priority: "normal"}, input.Query, "Added "+added.Title+".")
}

// writePanel answers with the panel region: the form and the list it owns, and
// nothing above them.
func writePanel(w http.ResponseWriter, r *http.Request, form FormState, query, note string) {
	matched := tasks.list(query)
	pw.WriteHTMLFragment(w, r, TaskPanel(TaskPanelParams{
		Form:       form,
		Tasks:      matched,
		EmptyLabel: emptyLabel(query, len(matched)),
		Note:       note,
	}))
}

type removeInput struct {
	ID    string `path:"id"`
	Query string `input:"q" check:"maxlen=40"`
}

// removeTask answers with the list region, so one response repairs the whole
// region rather than deleting a row the browser happens to be looking at. A row
// that is already gone is a 404: htmx leaves the list alone, which is the
// honest outcome for a click on a stale page.
func removeTask(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[removeInput](r)
	if err != nil {
		pw.WriteProblem(w, r, pw.BadRequest(err))
		return
	}
	if !tasks.remove(input.ID) {
		pw.WriteProblem(w, r, pw.NotFound("no such task"))
		return
	}
	matched := tasks.list(input.Query)
	pw.WriteHTMLFragment(w, r, TaskList(TaskListParams{
		Tasks:      matched,
		EmptyLabel: emptyLabel(input.Query, len(matched)),
	}))
}

// taskSummary hands the render a value that has not arrived yet, exactly as the
// page path would. What differs is the delivery: a fragment is always buffered,
// so the await boundary settles here and the body carries the counted markup
// with no placeholder, no boundary id, and nothing for a client runtime to do.
func taskSummary(w http.ResponseWriter, r *http.Request) {
	pw.WriteHTMLFragment(w, r, TaskSummary(TaskSummaryParams{
		Summary: pw.Go(r.Context(), summarize),
	}))
}

// brokenFragment is the guardrail, on purpose. StyledBadge declares a scoped
// style block, which folds into the document head — and a fragment response has
// no head. Dropping it would swap in an unstyled region with nothing in any
// log, so the framework answers 500 with a problem document instead.
func brokenFragment(w http.ResponseWriter, r *http.Request) {
	pw.WriteHTMLFragment(w, r, StyledBadge(StyledBadgeParams{Label: "styled by a scoped block"}))
}

func summarize(ctx context.Context) (Summary, error) {
	if err := sleep(ctx, 600*time.Millisecond); err != nil {
		return Summary{}, err
	}
	total, high := tasks.counts()
	return Summary{Total: total, High: high, Took: "600ms"}, nil
}

// validationFields reports the field-level failures behind a pw.Parse error.
// The distinction matters: those are worth showing next to an input, and
// anything else is not.
func validationFields(err error) ([]pw.FieldError, bool) {
	mapped, ok := httpbind.AsHTTPError(err)
	if !ok || len(mapped.Fields) == 0 {
		return nil, false
	}
	return mapped.Fields, true
}

func applyFieldErrors(form *FormState, fields []pw.FieldError) {
	for _, field := range fields {
		switch field.Field {
		case "title":
			form.TitleError = field.Message
		case "owner":
			form.OwnerError = field.Message
		default:
			// priority and q are set by the page rather than typed, so a failure
			// there means the request did not come from this form.
			form.FormError = field.Field + " " + field.Message
		}
	}
}

// knownPriority keeps the select on a value it actually offers. Anything else
// did not come from this form, and the rejection is already reported above it.
func knownPriority(value string) string {
	switch value {
	case "low", "normal", "high":
		return value
	default:
		return "normal"
	}
}

func emptyLabel(query string, matched int) string {
	switch {
	case matched > 0:
		return ""
	case query != "":
		return "Nothing matches “" + query + "”."
	default:
		return "No tasks yet."
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
