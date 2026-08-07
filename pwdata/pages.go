package pwdata

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
)

// view is everything a page renders from. One type rather than one per page,
// because every page carries the same chrome and the differences are which
// fields are set.
type view struct {
	// Prefix is where the console mounted this pane, or empty when the pane is
	// reached directly. Every link the pane writes carries it, because the
	// console strips it before the request arrives and an absolute path would
	// otherwise resolve against the console root and miss.
	Prefix      string
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
// from being empty everywhere except the page that happens to build it. This is
// also the request's one catalog query: a handler that needs the table list
// again reads view.Tables rather than asking the pool a second time.
func (s *Server) view(r *http.Request, section, title string) view {
	connection := s.connection(r)
	tables, err := connection.Tables(r.Context())
	reported := r.URL.Query().Get("error")
	if reported == "" {
		reported = errorText(err)
	}
	return view{
		Prefix:  r.Header.Get(panePrefixHeader),
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
.tabs { display:flex; gap:.15rem; border-bottom:1px solid var(--line); margin:.8rem 0 0; }
.tabs button { border:1px solid var(--line); border-bottom:0; border-radius:6px 6px 0 0; background:transparent;
  color:var(--muted); padding:.3rem .9rem; margin-bottom:-1px; }
.tabs button[aria-selected="true"] { background:var(--card); color:var(--fg); font-weight:600; border-bottom:1px solid var(--card); }
.panel { padding-top:.9rem; }
.panel[hidden] { display:none; }
table.grid th.sortable { cursor:pointer; user-select:none; }
table.grid th .dir { color:var(--muted); font-weight:400; margin-left:.2rem; }
table.grid td.rowacts { white-space:nowrap; }
table.grid td.rowacts button { padding:.05rem .35rem; font-size:11.5px; }
table.grid tr.blank td { background:color-mix(in srgb, var(--card) 60%, transparent); }
table.grid tr.blank input::placeholder { color:var(--muted); opacity:.6; }
table.grid td.dirty input, table.grid input.dirty { background:color-mix(in srgb, var(--bad) 18%, transparent); }
table.grid tr.dirty td:first-child { border-left:2px solid var(--bad); }
.filter { margin:.6rem 0; }
.filter input { font:inherit; padding:.25rem .5rem; border:1px solid var(--line); border-radius:4px;
  background:var(--bg); color:var(--fg); min-width:16rem; }
.savebar { position:sticky; top:0; z-index:5; background:var(--bg); padding:.5rem 0; border-bottom:1px solid var(--line);
  display:flex; gap:.7rem; align-items:center; }
.savebar button[disabled] { opacity:.45; cursor:default; }
a.back { display:block; color:var(--muted); text-decoration:none; font-size:12px; margin-bottom:.6rem; }
a.back:hover { color:var(--fg); }
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
{{if .Prefix}}<a class="back" href="/">&larr; pw dev console</a>{{end}}
<strong>data</strong>
<div class="env">{{.Engine}} · {{.Environment}}</div>
{{if gt (len .Connections) 1}}
<div class="group">connection</div>
{{range .Connections}}<a href="?c={{.Label}}"{{if eq $.Connection.Label .Label}} class="here"{{end}}>{{.Label}}{{if .ReadOnly}} <span class="fw">read-only</span>{{end}}</a>{{end}}
{{end}}
<a href="{{$.Prefix}}/console?c={{.Connection.Label}}"{{if eq .Section "console"}} class="here"{{end}}>statement console</a>
<a href="{{$.Prefix}}/queries?c={{.Connection.Label}}"{{if eq .Section "queries"}} class="here"{{end}}>declared queries</a>
<div class="group">tables</div>
{{range .Tables}}<a href="{{$.Prefix}}/table/{{.Name}}?c={{$.Connection.Label}}"{{if eq $.Title .Name}} class="here"{{end}}>{{if .Framework}}<span class="fw">{{.Name}}</span>{{else}}{{.Name}}{{end}}</a>{{end}}
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
		"inc":     func(a, b int) int { return a + b },
		"str":     strconv.Itoa,
		"keyJSON": keyJSON,
	}).Parse(body + resultBlock + chrome))
}

// keyJSON renders a row's primary key for the grid to address it by.
//
// The key travels with the row rather than being recomputed in the browser,
// because it is what identifies the row on the server and the browser has no
// business deciding what that is. A row whose key value is missing carries an
// empty object, and an edit to it is refused by the same check that refuses a
// keyless table.
func keyJSON(columns []Column, row []*string) template.JS {
	key := map[string]string{}
	for index, column := range columns {
		if column.PrimaryKey > 0 && index < len(row) && row[index] != nil {
			key[column.Name] = *row[index]
		}
	}
	encoded, err := json.Marshal(key)
	if err != nil {
		return template.JS("{}")
	}
	return template.JS(encoded)
}

var tablesPage = page(`{{define "body"}}
<h1>Tables</h1>
<p class="sub">{{len .Tables}} in the {{.Engine}} database this application opened, on <code>{{.Connection.Label}}</code>{{if .Connection.ReadOnly}} <span class="warn">(read-only replica)</span>{{end}}.</p>
{{with .Migration}}
<p class="sub">{{if .Present}}schema at version <code>{{.Version}}</code>, {{.Applied}} migration(s) applied{{else}}no migrations recorded on this database{{end}}</p>
{{end}}
<div class="wrap"><table class="grid">
<tr><th>table</th><th>owner</th></tr>
{{range .Tables}}<tr><td><a href="{{$.Prefix}}/table/{{.Name}}">{{.Name}}</a></td>
<td>{{if .Framework}}<span class="note">framework</span>{{else}}application{{end}}</td></tr>{{end}}
</table></div>
{{end}}`)

var tablePage = page(`{{define "blankrow"}}
<tr class="blank" data-new="1">
<td class="rowacts"><button class="rowsave" data-act="save" title="insert this row" hidden>add</button><button class="act" data-act="clear" title="clear this new row">clr</button></td>
{{range .Page.Columns}}<td data-column="{{.Name}}">
<input value="" data-original="" placeholder="{{.Type}}{{if gt .PrimaryKey 0}} · pk{{end}}"></td>{{end}}
</tr>
{{end}}
{{define "body"}}
{{$page := .Page}}
<h1>{{$page.Table}}</h1>
<p class="sub"><code>{{.Connection.Label}}</code> · {{.Engine}}{{if .Connection.ReadOnly}} · <span class="warn">read-only replica</span>{{end}}</p>

{{with $.Referenced}}<p class="note">Showing rows of <code>{{.Table}}</code> where <code>{{.Target}}</code> is <code>{{$.ReferencedValue}}</code>, followed from a foreign key. <a href="{{$.Prefix}}/table/{{.Table}}?c={{$.Connection.Label}}">show the whole table</a></p>{{end}}

<div class="tabs" role="tablist">
<button role="tab" aria-selected="true" data-panel="data">data</button>
<button role="tab" aria-selected="false" data-panel="schema">schema</button>
</div>

<div class="panel" id="panel-schema" hidden>
<div class="wrap"><table class="grid">
<tr><th>column</th><th>type</th><th>null</th><th>key</th><th>references</th></tr>
{{range $page.Columns}}{{$fk := index $.ForeignKeys .Name}}<tr>
<td><code>{{.Name}}</code></td><td>{{.Type}}</td>
<td>{{if .NotNull}}not null{{else}}<span class="note">nullable</span>{{end}}</td>
<td>{{if gt .PrimaryKey 0}}<span class="warn">pk {{.PrimaryKey}}</span>{{else}}<span class="note">—</span>{{end}}</td>
<td>{{if $fk.Table}}<a href="{{$.Prefix}}/table/{{$fk.Table}}?c={{$.Connection.Label}}"><code>{{$fk.Table}}.{{$fk.Target}}</code></a>{{else}}<span class="note">—</span>{{end}}</td>
</tr>{{end}}
</table></div>
</div>

<div class="panel" id="panel-data">
{{if not $page.Ordered}}<p class="note warn">This table has no primary key. Rows are paged by offset, their order is unspecified, and a single row cannot be edited here.</p>{{end}}
{{if $.Connection.ReadOnly}}<p class="note warn">This connection is a read-only replica, so rows are shown but cannot be edited here.</p>{{end}}

{{if and $.Keys (not $.Connection.ReadOnly)}}
<div class="savebar">
<button id="save" disabled>save all</button>
<span class="note" id="dirtycount">no changes</span>
</div>
{{end}}

<div class="filter"><input id="filter" type="search" placeholder="filter rows on this page" autocomplete="off">
<span class="note" id="shown"></span></div>

<div class="wrap"><table class="grid" id="rows"
 data-table="{{$page.Table}}" data-endpoint="{{$.Prefix}}/table/{{$page.Table}}/rows?c={{$.Connection.Label}}">
<thead><tr><th></th>
{{range $index, $column := $page.Columns}}<th class="sortable" data-index="{{$index}}"
 title="{{.Type}}{{if .NotNull}} · not null{{end}}{{if gt .PrimaryKey 0}} · primary key {{.PrimaryKey}}{{end}}">{{.Name}}<span class="dir"></span></th>{{end}}
</tr></thead>
<tbody>
{{if and $.Keys (not $.Connection.ReadOnly)}}{{template "blankrow" $}}{{end}}
{{range $rowIndex, $row := $page.Rows}}
<tr data-key='{{keyJSON $page.Columns $row}}'>
<td>{{if and $.Keys (not $.Connection.ReadOnly)}}<button class="danger act" data-act="delete">del</button>{{end}}</td>
{{range $index, $column := $page.Columns}}
{{$cell := index $row $index}}
{{$fk := index $.ForeignKeys $column.Name}}
<td{{if not $cell}} class="null"{{end}} data-column="{{$column.Name}}">
{{if and $.Keys (not $.Connection.ReadOnly)}}<input value="{{with $cell}}{{.}}{{end}}" placeholder="{{if not $cell}}NULL{{end}}" data-original="{{with $cell}}{{.}}{{end}}">
{{else}}{{if $cell}}{{$cell}}{{else}}NULL{{end}}{{end}}
{{if and $fk.Table $cell}}<a class="fk" title="{{$fk.Table}}.{{$fk.Target}}" href="{{$.Prefix}}/referenced/{{$fk.Table}}?c={{$.Connection.Label}}&amp;column={{$fk.Target}}&amp;value={{$cell}}">&rarr;</a>{{end}}
</td>
{{end}}
</tr>
{{end}}
{{if and $.Keys (not $.Connection.ReadOnly)}}{{template "blankrow" $}}{{end}}
</tbody>
</table></div>

<div class="bar">
{{if gt $page.Offset 0}}<a class="page" href="{{$.Prefix}}/table/{{$page.Table}}?c={{$.Connection.Label}}&offset=0">first</a>
<a class="page" href="{{$.Prefix}}/table/{{$page.Table}}?c={{$.Connection.Label}}&offset={{str $.PrevOffset}}">previous</a>{{end}}
{{if $page.More}}<a class="page" href="{{$.Prefix}}/table/{{$page.Table}}?c={{$.Connection.Label}}&offset={{str (inc $page.Offset $page.Limit)}}">next {{str $page.Limit}}</a>{{end}}
<span class="note">rows {{str (inc $page.Offset 1)}}–{{str (inc $page.Offset (len $page.Rows))}}</span>
</div>

</div>


<script>` + gridScript + `</script>
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
<td><a href="{{$.Prefix}}/query/{{.Package}}/{{.Name}}?c={{$.Connection.Label}}">{{.Name}}</a>{{if not .Exported}} <span class="note">unexported</span>{{end}}</td>
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
