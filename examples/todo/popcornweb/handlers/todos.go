package handlers

import (
	"net/http"
	"net/url"
	"strconv"

	"todo-popcornweb/queries"

	"github.com/shibukawa/popcornweb/pw"
)

func init() {
	mux.HandleFunc("GET /{$}", index)
	mux.HandleFunc("GET /api/todos", apiList)
	mux.HandleFunc("POST /todos", create)
	mux.HandleFunc("POST /todos/{id}/toggle", toggle)
	mux.HandleFunc("POST /todos/{id}/delete", remove)
}

// createInput is the form the add box posts. `required` and `maxlen` are
// checked by the generated binder, so the handler never sees an empty title.
type createInput struct {
	// Title is the text of the new item.
	Title string `payload:"title" check:"required,maxlen=200"`
}

// todoRef addresses one item. The mux pattern captures {id} as text; declaring
// it int here is what turns a non-numeric path into a 400 before the handler
// runs rather than a failed parse inside it.
type todoRef struct {
	// ID identifies the item to change.
	ID int `path:"id" check:"required,min=1"`
}

// todoListResponse is the JSON body of GET /api/todos.
type todoListResponse struct {
	Todos []apiTodo `json:"todos"`
}

type apiTodo struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

// load drains the query iterator into the record the template renders.
//
// The two Todo structs are distinct types on purpose: one belongs to the
// queries package and one to this template, and neither package can see the
// other's declaration. Copying between them is the seam where a column that
// changed shape stops the build.
func load(r *http.Request) ([]Todo, error) {
	todos := make([]Todo, 0, 32)
	for row, err := range queries.ListTodos(r.Context()) {
		if err != nil {
			return nil, err
		}
		id := strconv.Itoa(row.Id)
		todos = append(todos, Todo{
			Id: row.Id, Title: row.Title, Done: row.Done,
			ToggleURL: url.URL{Path: "/todos/" + id + "/toggle"},
			DeleteURL: url.URL{Path: "/todos/" + id + "/delete"},
		})
	}
	return todos, nil
}

// index renders the whole list.
func index(w http.ResponseWriter, r *http.Request) {
	todos, err := load(r)
	if err != nil {
		pw.WriteProblem(w, r, pw.InternalServerError(err))
		return
	}
	pw.WriteHTML(w, r, TodoPage(TodoPageParams{Todos: todos, Count: len(todos)}))
}

// apiList returns the same list as JSON.
func apiList(w http.ResponseWriter, r *http.Request) {
	todos, err := load(r)
	if err != nil {
		pw.WriteProblem(w, r, pw.InternalServerError(err))
		return
	}
	body := todoListResponse{Todos: make([]apiTodo, 0, len(todos))}
	for _, todo := range todos {
		body.Todos = append(body.Todos, apiTodo{ID: todo.Id, Title: todo.Title, Done: todo.Done})
	}
	pw.WriteAPI(w, r, body)
}

// create adds one item and returns to the list.
func create(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[createInput](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	if _, err := queries.CreateTodo(r.Context(), input.Title); err != nil {
		pw.WriteProblem(w, r, pw.InternalServerError(err))
		return
	}
	pw.RedirectSeeOther(w, r, "/")
}

// toggle flips one item between open and done.
func toggle(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[todoRef](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	if _, err := queries.ToggleTodo(r.Context(), input.ID); err != nil {
		pw.WriteProblem(w, r, pw.InternalServerError(err))
		return
	}
	pw.RedirectSeeOther(w, r, "/")
}

// remove deletes one item.
func remove(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[todoRef](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	if _, err := queries.DeleteTodo(r.Context(), input.ID); err != nil {
		pw.WriteProblem(w, r, pw.InternalServerError(err))
		return
	}
	pw.RedirectSeeOther(w, r, "/")
}
