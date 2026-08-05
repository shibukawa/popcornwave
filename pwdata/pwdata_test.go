package pwdata

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/shibukawa/popcornwave/database/sqlite"
	"github.com/shibukawa/tinybind-go/sqlbind"
)

func open(t *testing.T) *Server {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	schema := []string{
		`CREATE TABLE memos (id INTEGER PRIMARY KEY, title TEXT NOT NULL, body TEXT)`,
		`CREATE TABLE tags (memo_id INTEGER, name TEXT)`,
		`CREATE TABLE popcornwave_session (id TEXT PRIMARY KEY, payload TEXT)`,
		`INSERT INTO memos (id, title, body) VALUES (1, 'first', 'one'), (2, 'second', NULL)`,
		`INSERT INTO tags (memo_id, name) VALUES (1, 'work')`,
	}
	for _, statement := range schema {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
	return New(db, "sqlite", "dev")
}

func TestTablesAreListedAndFrameworkOnesMarked(t *testing.T) {
	tables, err := open(t).Tables(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, table := range tables {
		found[table.Name] = table.Framework
	}
	if _, ok := found["memos"]; !ok {
		t.Fatalf("tables = %v, want the application table", found)
	}
	if found["memos"] {
		t.Error("an application table was marked as framework-owned")
	}
	// The prefix is the rule an application reads its own schema by, so the
	// pane reads it the same way.
	if !found["popcornwave_session"] {
		t.Error("a framework table was not marked")
	}
}

func TestColumnsCarryTypesAndTheKey(t *testing.T) {
	columns, err := open(t).Columns(context.Background(), "memos")
	if err != nil {
		t.Fatal(err)
	}
	if len(columns) != 3 {
		t.Fatalf("columns = %+v, want three", columns)
	}
	if columns[0].PrimaryKey != 1 {
		t.Errorf("id = %+v, want the first key column", columns[0])
	}
	if !columns[1].NotNull {
		t.Errorf("title = %+v, want not null", columns[1])
	}
}

func TestRowsArePagedAndOrderedByKey(t *testing.T) {
	page, err := open(t).Rows(context.Background(), "memos", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !page.Ordered {
		t.Error("a keyed table was not reported as ordered")
	}
	if len(page.Rows) != 2 {
		t.Fatalf("rows = %v, want two", page.Rows)
	}
	// NULL and the empty string are the distinction a developer opens this
	// pane to check, so they must not render the same.
	if page.Rows[1][2] != nil {
		t.Errorf("body = %v, want NULL preserved as nil", *page.Rows[1][2])
	}
}

func TestTableWithoutAKeySaysItsOrderIsUnspecified(t *testing.T) {
	page, err := open(t).Rows(context.Background(), "tags", 0)
	if err != nil {
		t.Fatal(err)
	}
	if page.Ordered {
		t.Error("a table with no primary key claimed a stable order")
	}
}

// The catalog is the allow list. A name carrying SQL is refused rather than
// quoted and hoped about.
func TestUnknownTableIsRefused(t *testing.T) {
	_, err := open(t).Rows(context.Background(), "memos; DROP TABLE memos", 0)
	if err == nil {
		t.Fatal("a table name the catalog does not report was accepted")
	}
	if !strings.Contains(err.Error(), "no table named") {
		t.Errorf("err = %v, want the name refused", err)
	}
}

func TestUpdateRowWritesByPrimaryKey(t *testing.T) {
	server := open(t)
	affected, err := server.UpdateRow(context.Background(), RowEdit{
		Table:  "memos",
		Key:    map[string]string{"id": "1"},
		Values: map[string]string{"title": "renamed"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if affected != 1 {
		t.Errorf("affected = %d, want 1", affected)
	}
	page, _ := server.Rows(context.Background(), "memos", 0)
	if *page.Rows[0][1] != "renamed" {
		t.Errorf("title = %q, want the new value", *page.Rows[0][1])
	}
}

func TestUpdateCanSetAColumnToNull(t *testing.T) {
	server := open(t)
	if _, err := server.UpdateRow(context.Background(), RowEdit{
		Table: "memos", Key: map[string]string{"id": "1"}, Nulls: []string{"body"},
	}); err != nil {
		t.Fatal(err)
	}
	page, _ := server.Rows(context.Background(), "memos", 0)
	if page.Rows[0][2] != nil {
		t.Errorf("body = %q, want NULL", *page.Rows[0][2])
	}
}

func TestInsertAndDeleteRow(t *testing.T) {
	server := open(t)
	if _, err := server.InsertRow(context.Background(), RowEdit{
		Table:  "memos",
		Values: map[string]string{"id": "3", "title": "third"},
	}); err != nil {
		t.Fatal(err)
	}
	page, _ := server.Rows(context.Background(), "memos", 0)
	if len(page.Rows) != 3 {
		t.Fatalf("rows = %d, want three after the insert", len(page.Rows))
	}
	if _, err := server.DeleteRow(context.Background(), RowEdit{
		Table: "memos", Key: map[string]string{"id": "3"},
	}); err != nil {
		t.Fatal(err)
	}
	page, _ = server.Rows(context.Background(), "memos", 0)
	if len(page.Rows) != 2 {
		t.Errorf("rows = %d, want two after the delete", len(page.Rows))
	}
}

// Without a key there is no way to address one row, and saying so beats
// changing an unknown number of them.
func TestEditingAKeylessTableIsRefused(t *testing.T) {
	_, err := open(t).UpdateRow(context.Background(), RowEdit{
		Table: "tags", Values: map[string]string{"name": "x"},
	})
	if err == nil || !strings.Contains(err.Error(), "no primary key") {
		t.Errorf("err = %v, want the missing key explained", err)
	}
}

func TestUnknownColumnIsRefused(t *testing.T) {
	_, err := open(t).UpdateRow(context.Background(), RowEdit{
		Table: "memos", Key: map[string]string{"id": "1"},
		Values: map[string]string{"nope": "x"},
	})
	if err == nil || !strings.Contains(err.Error(), "no column named") {
		t.Errorf("err = %v, want the column refused", err)
	}
}

func TestConsoleRunsAStatementAndReportsRows(t *testing.T) {
	result := open(t).Exec(context.Background(), "SELECT id, title FROM memos ORDER BY id")
	if result.Error != "" {
		t.Fatalf("error = %s", result.Error)
	}
	if !result.Returned || len(result.Rows) != 2 {
		t.Fatalf("result = %+v, want two rows", result)
	}
}

func TestConsoleReportsAffectedRowsForAWrite(t *testing.T) {
	result := open(t).Exec(context.Background(), "UPDATE memos SET title = 'x'")
	if result.Error != "" {
		t.Fatalf("error = %s", result.Error)
	}
	if result.Returned {
		t.Error("a write was reported as returning rows")
	}
	if result.Affected != 2 {
		t.Errorf("affected = %d, want 2", result.Affected)
	}
}

func TestConsoleReportsAFailingStatement(t *testing.T) {
	result := open(t).Exec(context.Background(), "SELECT * FROM nope")
	if result.Error == "" {
		t.Error("a failing statement was reported as success")
	}
}

func TestDeclaredQueryRunsTheGeneratedStatement(t *testing.T) {
	registry.Lock()
	registry.queries = nil
	registry.Unlock()
	RegisterQuery(Query{
		Package: "queries", Name: "findMemo", Exported: false,
		Params: []Param{{Name: "id", Kind: "int"}},
		Build: func(args []string) (sqlbind.Statement, error) {
			builder := sqlbind.NewBuilder(sqlbind.Question)
			builder.WriteString("SELECT title FROM memos WHERE id = ")
			builder.Arg(args[0])
			return builder.Statement(), nil
		},
	})
	result := open(t).RunQuery(context.Background(), "queries", "findMemo", []string{"2"})
	if result.Error != "" {
		t.Fatalf("error = %s", result.Error)
	}
	if len(result.Rows) != 1 || *result.Rows[0][0] != "second" {
		t.Fatalf("result = %+v, want the second memo", result)
	}
	// The statement shown is the one the generated builder produced, so what
	// the pane ran and what the application would run are the same text.
	if !strings.Contains(result.SQL, "FROM memos") {
		t.Errorf("SQL = %q, want the built statement", result.SQL)
	}
}

func TestPagesRender(t *testing.T) {
	server := open(t)
	registry.Lock()
	registry.queries = nil
	registry.Unlock()
	for _, path := range []string{"/", "/table/memos", "/table/tags", "/console", "/queries"} {
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK {
			t.Errorf("%s: status = %d", path, recorder.Code)
			continue
		}
		if body := recorder.Body.String(); strings.Contains(body, "template error") {
			t.Errorf("%s failed to render:\n%s", path, body)
		}
	}
}

func TestEditFormAppliesAnUpdate(t *testing.T) {
	server := open(t)
	form := url.Values{
		"action":      {"update"},
		"key.id":      {"1"},
		"value.title": {"through the form"},
		"offset":      {"0"},
	}
	request := httptest.NewRequest(http.MethodPost, "/table/memos/row", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want a redirect back to the table", recorder.Code)
	}
	page, _ := server.Rows(context.Background(), "memos", 0)
	if *page.Rows[0][1] != "through the form" {
		t.Errorf("title = %q, want the posted value", *page.Rows[0][1])
	}
}

func TestDialectsDifferWhereTheyMust(t *testing.T) {
	for _, engine := range []struct {
		driver, quoted, marker string
	}{
		{"sqlite", `"x"`, "?"},
		{"postgres", `"x"`, "$1"},
		{"pgx", `"x"`, "$1"},
		{"mysql", "`x`", "?"},
	} {
		d := dialectFor(engine.driver)
		if got := d.quote("x"); got != engine.quoted {
			t.Errorf("%s quote = %s, want %s", engine.driver, got, engine.quoted)
		}
		if got := d.placeholder(1); got != engine.marker {
			t.Errorf("%s placeholder = %s, want %s", engine.driver, got, engine.marker)
		}
		if d.tables == "" || d.columns == "" {
			t.Errorf("%s has no catalog statements", engine.driver)
		}
	}
}
