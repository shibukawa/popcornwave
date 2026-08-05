package pwdata

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// Handler serves the data pane: schema, rows, edits, a statement console, and
// the declared queries the project generated.
//
// It is mounted by the framework on a loopback listener of its own, never on
// the application's, so nothing here is reachable from the port the application
// serves.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.pageTables)
	mux.HandleFunc("GET /table/{name}", s.pageTable)
	mux.HandleFunc("POST /table/{name}/row", s.editRow)
	mux.HandleFunc("GET /console", s.pageConsole)
	mux.HandleFunc("POST /console", s.pageConsole)
	mux.HandleFunc("GET /referenced/{table}", s.pageReferenced)
	mux.HandleFunc("GET /queries", s.pageQueries)
	mux.HandleFunc("GET /query/{package}/{name}", s.pageQuery)
	mux.HandleFunc("POST /query/{package}/{name}", s.pageQuery)
	mux.HandleFunc("GET /api/tables", s.apiTables)
	return mux
}

// connection resolves which pool a request addresses. The label travels as a
// query parameter so every link and form carries it, and an unknown one falls
// back to the default rather than erroring on a stale bookmark.
func (s *Server) connection(r *http.Request) *Connection {
	return s.Lookup(r.URL.Query().Get("c"))
}

func (s *Server) apiTables(w http.ResponseWriter, r *http.Request) {
	tables, err := s.connection(r).Tables(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tables)
}

func (s *Server) pageTables(w http.ResponseWriter, r *http.Request) {
	view := s.view(r, "tables", "tables")
	if state, err := s.connection(r).MigrationState(r.Context()); err == nil {
		view.Migration = &state
	}
	s.render(w, tablesPage, view)
}

func (s *Server) pageTable(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	connection := s.connection(r)
	view := s.view(r, "tables", name)
	page, err := connection.Rows(r.Context(), name, offset)
	view.Page = &page
	view.ForeignKeys = connection.ForeignKeys(r.Context(), name)
	view.Error = errorText(err)
	view.Keys = primaryKey(page.Columns)
	view.PrevOffset = max(0, offset-pageSize)
	s.render(w, tablePage, view)
}

// editRow applies one row change and returns to the table it changed, so the
// result is the table as it now stands rather than a message about it.
func (s *Server) editRow(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	edit := RowEdit{Table: name, Key: map[string]string{}, Values: map[string]string{}}
	for field, values := range r.Form {
		switch {
		case strings.HasPrefix(field, "key."):
			edit.Key[strings.TrimPrefix(field, "key.")] = values[0]
		case strings.HasPrefix(field, "value."):
			edit.Values[strings.TrimPrefix(field, "value.")] = values[0]
		case field == "null":
			edit.Nulls = values
		}
	}
	// A column explicitly set to NULL must not also arrive as an empty string.
	for _, name := range edit.Nulls {
		delete(edit.Values, name)
	}
	connection := s.connection(r)
	var affected int64
	var err error
	switch {
	case connection.ReadOnly:
		// Not a rule the pane applies, but what the connection is. Saying so
		// beats an engine error the developer has to translate.
		err = errReadOnlyConnection
	case r.FormValue("action") == "insert":
		affected, err = connection.InsertRow(r.Context(), edit)
	case r.FormValue("action") == "delete":
		affected, err = connection.DeleteRow(r.Context(), edit)
	default:
		affected, err = connection.UpdateRow(r.Context(), edit)
	}
	target := "/table/" + name + "?c=" + connection.Label + "&offset=" + r.FormValue("offset")
	if err != nil {
		target += "&error=" + urlValue(err.Error())
	} else {
		target += "&changed=" + strconv.FormatInt(affected, 10)
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// pageReferenced follows one foreign key. The column and table came from the
// catalog and the value travels as a bind parameter, so this is a selection
// rather than a filter the page composed.
func (s *Server) pageReferenced(w http.ResponseWriter, r *http.Request) {
	connection := s.connection(r)
	table := r.PathValue("table")
	column := r.URL.Query().Get("column")
	value := r.URL.Query().Get("value")
	view := s.view(r, "tables", table)
	page, err := connection.Referenced(r.Context(), table, column, value)
	view.Page = &page
	view.Referenced = &ForeignKey{Column: column, Table: table, Target: column}
	view.ReferencedValue = value
	view.ForeignKeys = connection.ForeignKeys(r.Context(), table)
	view.Error = errorText(err)
	view.Keys = primaryKey(page.Columns)
	s.render(w, tablePage, view)
}

func (s *Server) pageConsole(w http.ResponseWriter, r *http.Request) {
	view := s.view(r, "console", "statement console")
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		statement := r.FormValue("statement")
		view.Statement = statement
		connection := s.connection(r)
		if r.FormValue("action") == "explain" {
			result := connection.Explain(r.Context(), statement)
			view.Result = &result
		} else {
			result := connection.Exec(r.Context(), statement)
			view.Result = &result
		}
	}
	s.render(w, consolePage, view)
}

func (s *Server) pageQueries(w http.ResponseWriter, r *http.Request) {
	view := s.view(r, "queries", "declared queries")
	s.render(w, queriesPage, view)
}

func (s *Server) pageQuery(w http.ResponseWriter, r *http.Request) {
	pkg, name := r.PathValue("package"), r.PathValue("name")
	query, ok := lookupQuery(pkg, name)
	if !ok {
		http.NotFound(w, r)
		return
	}
	view := s.view(r, "queries", name)
	view.Query = &query
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		args := make([]string, len(query.Params))
		for index, param := range query.Params {
			args[index] = r.FormValue("arg." + param.Name)
		}
		view.Args = args
		connection := s.connection(r)
		if r.FormValue("action") == "explain" {
			result := connection.ExplainQuery(r.Context(), pkg, name, args)
			view.Result = &result
		} else {
			result := connection.RunQuery(r.Context(), pkg, name, args)
			view.Result = &result
		}
	}
	s.render(w, queryPage, view)
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func urlValue(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "&", "%26"), " ", "+")
}
