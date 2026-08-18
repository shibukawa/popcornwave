// Command todo-stdhttp is the control in the comparison: the same todo
// application written the way a Go developer writes one today, with net/http,
// html/template, encoding/json, and pgx.
//
// It is deliberately unremarkable. Every choice here is the obvious one, so
// that whatever the measurements show is a property of the approach rather than
// of an artificially weak baseline: the template is parsed once at startup, the
// pool is shared, and the queries are prepared statements sent through pgx's
// binary protocol.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

type Todo struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

// todoListResponse wraps the array so that GET /api/todos answers with the same
// body shape as the Popcorn Web service, which is what makes the two
// comparable under load.
type todoListResponse struct {
	Todos []Todo `json:"todos"`
}

// page is parsed once, at startup, exactly as a net/http application would do
// it. Parsing is therefore not part of what any request pays.
var page = template.Must(template.New("todos").Parse(
	`<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>Todo</title></head>
<body>
<h1>Todo</h1>
<form method="post" action="/todos">
  <input name="title" autocomplete="off" required>
  <button type="submit">Add</button>
</form>
<ul>{{range .}}<li class="{{if .Done}}done{{else}}open{{end}}">
  <span class="title">{{.Title}}</span>
  <form method="post" action="/todos/{{.ID}}/toggle"><button type="submit">toggle</button></form>
  <form method="post" action="/todos/{{.ID}}/delete"><button type="submit">delete</button></form>
</li>{{end}}</ul>
<p class="count">{{len .}} items</p>
</body>
</html>`))

type app struct{ db store }

func (a *app) index(w http.ResponseWriter, r *http.Request) {
	todos, err := a.db.list(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := page.Execute(w, todos); err != nil {
		log.Printf("render: %v", err)
	}
}

func (a *app) apiList(w http.ResponseWriter, r *http.Request) {
	todos, err := a.db.list(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(todoListResponse{Todos: todos}); err != nil {
		log.Printf("encode: %v", err)
	}
}

func (a *app) create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	title := r.PostFormValue("title")
	if title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	if err := a.db.create(r.Context(), title); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// id reads the wildcard the Go 1.22 mux captured. A pattern cannot express
// "digits only", so the handler still has to reject everything else.
func id(r *http.Request) (int64, bool) {
	v, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return v, err == nil && v > 0
}

func (a *app) toggle(w http.ResponseWriter, r *http.Request) {
	todoID, ok := id(r)
	if !ok {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := a.db.toggle(r.Context(), todoID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *app) remove(w http.ResponseWriter, r *http.Request) {
	todoID, ok := id(r)
	if !ok {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := a.db.remove(r.Context(), todoID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// requestID is the one piece of middleware, present so that both services wrap
// their mux the same way and the comparison is not measuring its absence.
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", strconv.FormatInt(time.Now().UnixNano(), 36))
		next.ServeHTTP(w, r)
	})
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://todo:todo@127.0.0.1:5433/todo?sslmode=disable"
	}
	db, layer, err := openStore(ctx, url)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer db.Close()

	servePprof()

	a := &app{db: db}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", a.index)
	mux.HandleFunc("GET /api/todos", a.apiList)
	mux.HandleFunc("POST /todos", a.create)
	mux.HandleFunc("POST /todos/{id}/toggle", a.toggle)
	mux.HandleFunc("POST /todos/{id}/delete", a.remove)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8081"
	}
	server := &http.Server{Addr: addr, Handler: requestID(mux)}

	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()

	log.Printf("todo-stdhttp listening on %s (db layer: %s)", addr, layer)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("serve: %v", err)
	}
}
