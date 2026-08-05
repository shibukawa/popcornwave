package pwdata

import (
	"html/template"
	"net/http"
	"strconv"
)

// view is everything a page renders from. One type rather than one per page,
// because every page carries the same chrome and the differences are which
// fields are set.
type view struct {
	Section     string
	Title       string
	Engine      string
	Environment string
	Tables      []Table
	Page        *Page
	Keys        []Column
	Query       *Query
	Queries     []Query
	Args        []string
	Statement   string
	Result      *Result
	Connection  *Connection
	Connections []Connection
	ForeignKeys map[string]ForeignKey
	// Referenced is set on a page reached by following a foreign key, so it can
	// say what it is showing and offer the way back.
	Referenced      *ForeignKey
	ReferencedValue string
	Migration       *Migration
	Error           string
	Changed         string
	// PrevOffset is the previous page's offset, clamped at zero. It is
	// computed here rather than in the template, where arithmetic reads worse
	// than it works.
	PrevOffset int
}

// view carries the sidebar as well as the page, because every page repeats it.
// Listing the tables here rather than in one handler is what keeps the sidebar
// from being empty everywhere except the page that happens to build it.
func (s *Server) view(r *http.Request, section, title string) view {
	connection := s.connection(r)
	tables, err := connection.Tables(r.Context())
	reported := r.URL.Query().Get("error")
	if reported == "" {
		reported = errorText(err)
	}
	return view{
		Tables:  tables,
		Error:   reported,
		Section: section, Title: title,
		Engine: connection.Engine(), Environment: s.environment,
		Connection: connection, Connections: s.connections,
		Queries: Queries(),
		Changed: r.URL.Query().Get("changed"),
	}
}

func (s *Server) render(w http.ResponseWriter, page *template.Template, data view) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := page.Execute(w, data); err != nil {
		_, _ = w.Write([]byte("<p>data pane template error: " + template.HTMLEscapeString(err.Error()) + "</p>"))
	}
}

const style = `
:root { color-scheme: light dark; --bg:#fff; --fg:#1a1a1a; --muted:#666; --line:#e3e3e3; --card:#fafafa; --ok:#177245; --bad:#b3261e; --warn:#8a6d00; }
@media (prefers-color-scheme: dark) {
  :root { --bg:#141414; --fg:#e8e8e8; --muted:#9a9a9a; --line:#2c2c2c; --card:#1c1c1c; --ok:#5fbf88; --bad:#f2b8b5; --warn:#e0c060; }
}
* { box-sizing: border-box; }
body { margin:0; background:var(--bg); color:var(--fg); font:14px/1.55 ui-sans-serif,system-ui,-apple-system,"Segoe UI",sans-serif; }
.layout { display:flex; min-height:100vh; }
aside { width:15rem; border-right:1px solid var(--line); padding:1rem .8rem; flex:none; overflow:auto; }
main { flex:1; padding:1.1rem 1.4rem 4rem; min-width:0; overflow:auto; }
aside strong { display:block; margin-bottom:.2rem; }
aside .env { color:var(--muted); font-size:12px; margin-bottom:.8rem; }
aside .group { color:var(--muted); font-size:11px; text-transform:uppercase; letter-spacing:.05em; margin:1rem 0 .3rem; }
aside a { display:block; padding:.15rem 0; color:var(--fg); text-decoration:none; font-size:13px; }
aside a:hover { text-decoration:underline; }
aside a.here { font-weight:700; }
aside .fw { color:var(--muted); }
h1 { font-size:1.15rem; margin:0 0 .1rem; }
p.sub { color:var(--muted); margin:0 0 1rem; font-size:13px; }
table.grid { border-collapse:collapse; width:100%; font-size:12.5px; font-variant-numeric:tabular-nums; }
table.grid th, table.grid td { border:1px solid var(--line); padding:.25rem .45rem; text-align:left; vertical-align:top; max-width:22rem; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
table.grid th { background:var(--card); font-weight:600; white-space:nowrap; }
table.grid td.null { color:var(--muted); font-style:italic; }
table.grid input { width:100%; border:1px solid transparent; background:transparent; color:inherit; font:inherit; padding:.1rem .15rem; }
table.grid input:focus { border-color:var(--line); background:var(--bg); outline:none; }
.wrap { overflow-x:auto; border-radius:6px; }
.bar { display:flex; gap:.6rem; align-items:center; margin:.7rem 0; flex-wrap:wrap; }
button { font:inherit; padding:.2rem .6rem; border:1px solid var(--line); background:var(--card); color:var(--fg); border-radius:4px; cursor:pointer; }
button.danger { color:var(--bad); }
a.page { color:var(--fg); text-decoration:none; border:1px solid var(--line); border-radius:4px; padding:.2rem .6rem; }
textarea { width:100%; min-height:8rem; font:12.5px ui-monospace,SFMono-Regular,Menlo,monospace; padding:.6rem; border:1px solid var(--line); border-radius:6px; background:var(--card); color:var(--fg); }
pre { background:var(--card); border:1px solid var(--line); border-radius:6px; padding:.6rem; overflow-x:auto; font-size:12.5px; margin:.5rem 0; }
.note { color:var(--muted); font-size:12.5px; }
.bad { color:var(--bad); }
.ok { color:var(--ok); }
.warn { color:var(--warn); }
code { font-family:ui-monospace,SFMono-Regular,Menlo,monospace; font-size:12.5px; }
a.fk { text-decoration:none; color:var(--muted); margin-left:.25rem; }
a.fk:hover { color:var(--fg); }
label { font-size:12.5px; color:var(--muted); display:block; margin:.5rem 0 .1rem; }
input.text { font:inherit; padding:.25rem .4rem; border:1px solid var(--line); border-radius:4px; background:var(--bg); color:var(--fg); min-width:18rem; }
`

const chrome = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Title}} — pw dev data</title><style>` + style + `</style></head>
<body><div class="layout">
<aside>
<strong>data</strong>
<div class="env">{{.Engine}} · {{.Environment}}</div>
{{if gt (len .Connections) 1}}
<div class="group">connection</div>
{{range .Connections}}<a href="?c={{.Label}}"{{if eq $.Connection.Label .Label}} class="here"{{end}}>{{.Label}}{{if .ReadOnly}} <span class="fw">read-only</span>{{end}}</a>{{end}}
{{end}}
<a href="/console?c={{.Connection.Label}}"{{if eq .Section "console"}} class="here"{{end}}>statement console</a>
<a href="/queries?c={{.Connection.Label}}"{{if eq .Section "queries"}} class="here"{{end}}>declared queries</a>
<div class="group">tables</div>
{{range .Tables}}<a href="/table/{{.Name}}?c={{$.Connection.Label}}"{{if eq $.Title .Name}} class="here"{{end}}>{{if .Framework}}<span class="fw">{{.Name}}</span>{{else}}{{.Name}}{{end}}</a>{{end}}
</aside>
<main>
{{if .Error}}<p class="bad">{{.Error}}</p>{{end}}
{{if .Changed}}<p class="ok">{{.Changed}} row(s) changed</p>{{end}}
{{template "body" .}}
</main></div></body></html>`

var resultBlock = `
{{define "result"}}
{{with .}}
{{if .Error}}<p class="bad">{{.Error}}</p>{{end}}
{{if .SQL}}<pre>{{.SQL}}</pre>{{end}}
{{if .Returned}}
<div class="wrap"><table class="grid">
<tr>{{range .Columns}}<th>{{.}}</th>{{end}}</tr>
{{range .Rows}}<tr>{{range .}}{{if .}}<td>{{.}}</td>{{else}}<td class="null">NULL</td>{{end}}{{end}}</tr>{{end}}
</table></div>
<p class="note">{{len .Rows}} row(s){{if .Truncated}} · truncated; add your own LIMIT to see further{{end}}</p>
{{else if not .Error}}
<p class="note">{{if lt .Affected 0}}the driver did not report a row count{{else}}{{.Affected}} row(s) affected{{end}}</p>
{{end}}
{{end}}
{{end}}`

func page(body string) *template.Template {
	return template.Must(template.New("page").Funcs(template.FuncMap{
		"inc": func(a, b int) int { return a + b },
		"str": strconv.Itoa,
	}).Parse(body + resultBlock + chrome))
}

var tablesPage = page(`{{define "body"}}
<h1>Tables</h1>
<p class="sub">{{len .Tables}} in the {{.Engine}} database this application opened, on <code>{{.Connection.Label}}</code>{{if .Connection.ReadOnly}} <span class="warn">(read-only replica)</span>{{end}}.</p>
{{with .Migration}}
<p class="sub">{{if .Present}}schema at version <code>{{.Version}}</code>, {{.Applied}} migration(s) applied{{else}}no migrations recorded on this database{{end}}</p>
{{end}}
<div class="wrap"><table class="grid">
<tr><th>table</th><th>owner</th></tr>
{{range .Tables}}<tr><td><a href="/table/{{.Name}}">{{.Name}}</a></td>
<td>{{if .Framework}}<span class="note">framework</span>{{else}}application{{end}}</td></tr>{{end}}
</table></div>
{{end}}`)

var tablePage = page(`{{define "body"}}
{{$page := .Page}}
<h1>{{$page.Table}}</h1>
<p class="sub">
{{range $page.Columns}}<code>{{.Name}}</code> {{.Type}}{{if gt .PrimaryKey 0}} <span class="warn">pk</span>{{end}}{{if .NotNull}} not null{{end}} · {{end}}
</p>
{{with $.Referenced}}<p class="note">Showing rows of <code>{{.Table}}</code> where <code>{{.Target}}</code> is <code>{{$.ReferencedValue}}</code>, followed from a foreign key. <a href="/table/{{.Table}}?c={{$.Connection.Label}}">show the whole table</a></p>{{end}}
{{if $.Connection.ReadOnly}}<p class="note warn">This connection is a read-only replica, so rows are shown but cannot be edited here.</p>{{end}}
{{if not $page.Ordered}}<p class="note warn">This table has no primary key. Rows are paged by offset, their order is unspecified, and a single row cannot be edited here.</p>{{end}}

<div class="wrap"><table class="grid">
<tr><th></th>{{range $page.Columns}}<th>{{.Name}}</th>{{end}}</tr>
{{range $rowIndex, $row := $page.Rows}}
<tr>
<td>
{{if and $.Keys (not $.Connection.ReadOnly)}}
<form method="post" action="/table/{{$page.Table}}/row?c={{$.Connection.Label}}" id="row{{$rowIndex}}">
<input type="hidden" name="offset" value="{{str $page.Offset}}">
{{range $index, $column := $page.Columns}}{{if gt $column.PrimaryKey 0}}
<input type="hidden" name="key.{{$column.Name}}" value="{{with index $row $index}}{{.}}{{end}}">
{{end}}{{end}}
<button name="action" value="update" title="save this row">save</button>
<button name="action" value="delete" class="danger" title="delete this row">del</button>
</form>
{{end}}
</td>
{{range $index, $column := $page.Columns}}
{{$cell := index $row $index}}
{{$fk := index $.ForeignKeys $column.Name}}
<td{{if not $cell}} class="null"{{end}}>
{{if and $.Keys (not $.Connection.ReadOnly)}}<input form="row{{$rowIndex}}" name="value.{{$column.Name}}" value="{{with $cell}}{{.}}{{end}}" placeholder="{{if not $cell}}NULL{{end}}">
{{else}}{{if $cell}}{{$cell}}{{else}}NULL{{end}}{{end}}
{{if and $fk.Table $cell}}<a class="fk" title="{{$fk.Table}}.{{$fk.Target}}" href="/referenced/{{$fk.Table}}?c={{$.Connection.Label}}&amp;column={{$fk.Target}}&amp;value={{$cell}}">&rarr;</a>{{end}}
</td>
{{end}}
</tr>
{{end}}
</table></div>

<div class="bar">
{{if gt $page.Offset 0}}<a class="page" href="/table/{{$page.Table}}?c={{$.Connection.Label}}&offset=0">first</a>
<a class="page" href="/table/{{$page.Table}}?c={{$.Connection.Label}}&offset={{str $.PrevOffset}}">previous</a>{{end}}
{{if $page.More}}<a class="page" href="/table/{{$page.Table}}?c={{$.Connection.Label}}&offset={{str (inc $page.Offset $page.Limit)}}">next {{str $page.Limit}}</a>{{end}}
<span class="note">rows {{str (inc $page.Offset 1)}}–{{str (inc $page.Offset (len $page.Rows))}}</span>
</div>

{{if $.Keys}}
<h1 style="font-size:1rem;margin-top:2rem">Insert a row</h1>
<form method="post" action="/table/{{$page.Table}}/row?c={{$.Connection.Label}}">
<input type="hidden" name="offset" value="{{str $page.Offset}}">
{{range $page.Columns}}
<label>{{.Name}} <span class="note">{{.Type}}</span></label>
<input class="text" name="value.{{.Name}}" placeholder="leave blank to use the column default">
{{end}}
<div class="bar"><button name="action" value="insert">insert</button></div>
</form>
{{end}}
{{end}}`)

var consolePage = page(`{{define "body"}}
<h1>Statement console</h1>
<p class="sub">Runs against the pool the application opened, on the {{.Engine}} database it is serving from.</p>
<form method="post">
<textarea name="statement" placeholder="select * from ...">{{.Statement}}</textarea>
<div class="bar"><button name="action" value="run">run</button>
<button name="action" value="explain">explain</button>
<span class="note">one statement per run · results are capped, writes are not · explain reads the plan without running it</span></div>
</form>
{{template "result" .Result}}
{{end}}`)

var queriesPage = page(`{{define "body"}}
<h1>Declared queries</h1>
<p class="sub">{{len .Queries}} generated from this project's .pw.sql sources. Running one here builds the same statement the application builds.</p>
{{if not .Queries}}<p class="note">This project declares none, or none have been generated yet.</p>{{end}}
<div class="wrap"><table class="grid">
<tr><th>query</th><th>package</th><th>parameters</th></tr>
{{range .Queries}}<tr>
<td><a href="/query/{{.Package}}/{{.Name}}?c={{$.Connection.Label}}">{{.Name}}</a>{{if not .Exported}} <span class="note">unexported</span>{{end}}</td>
<td>{{.Package}}</td>
<td>{{range .Params}}<code>{{.Name}}</code> {{.Kind}} {{end}}{{if not .Params}}<span class="note">none</span>{{end}}</td>
</tr>{{end}}
</table></div>
{{end}}`)

var queryPage = page(`{{define "body"}}
{{$q := .Query}}
<h1>{{$q.Name}}</h1>
<p class="sub"><code>{{$q.Package}}</code>{{if not $q.Exported}} · unexported, reachable only from inside its own package{{end}}</p>
<form method="post">
{{range $index, $param := $q.Params}}
<label>{{$param.Name}} <span class="note">{{$param.Kind}}</span></label>
<input class="text" name="arg.{{$param.Name}}" value="{{if $.Args}}{{index $.Args $index}}{{end}}">
{{end}}
{{if not $q.Params}}<p class="note">This query takes no parameters.</p>{{end}}
<div class="bar"><button name="action" value="run">run</button>
<button name="action" value="explain">explain</button>
<span class="note">explain builds the same statement and reads its plan without running it</span></div>
</form>
{{template "result" .Result}}
{{end}}`)
