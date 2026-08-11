//go:build !fasthttp

package handlers

import (
	"net/http"

	"github.com/shibukawa/popcornwave/pw"
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
			Title:    pw.FormValue(r, "title"),
			Owner:    pw.FormValue(r, "owner"),
			Priority: knownPriority(pw.FormValue(r, "priority")),
		}
		applyFieldErrors(&form, fields)
		writePanel(w, r, form, pw.FormValue(r, "q"), "")
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
