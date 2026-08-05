package devconsole

import (
	"html/template"
	"net/http"
	"strings"
	"time"
)

// style is the whole console stylesheet. It is inline on every page because the
// console is one binary serving a handful of documents to one reader on
// loopback, and a separate asset would buy caching nobody is waiting on.
const style = `
:root { color-scheme: light dark; --bg:#fff; --fg:#1a1a1a; --muted:#666; --line:#e3e3e3; --card:#fafafa; --ok:#177245; --bad:#b3261e; --wait:#8a6d00; }
@media (prefers-color-scheme: dark) {
  :root { --bg:#141414; --fg:#e8e8e8; --muted:#9a9a9a; --line:#2c2c2c; --card:#1c1c1c; --ok:#5fbf88; --bad:#f2b8b5; --wait:#e0c060; }
}
* { box-sizing: border-box; }
body { margin:0; background:var(--bg); color:var(--fg); font:14px/1.6 ui-sans-serif,system-ui,-apple-system,"Segoe UI",sans-serif; }
main { max-width: 60rem; margin: 0 auto; padding: 1.5rem 1.25rem 4rem; }
nav { border-bottom:1px solid var(--line); padding:.6rem 1.25rem; display:flex; gap:1rem; align-items:baseline; flex-wrap:wrap; }
nav .brand { font-weight:600; letter-spacing:.02em; }
nav a { color:var(--muted); text-decoration:none; }
nav a:hover, nav a.here { color:var(--fg); }
h1 { font-size:1.35rem; margin:1.2rem 0 .2rem; }
h2 { font-size:1rem; margin:2rem 0 .6rem; font-weight:600; }
p.sub { color:var(--muted); margin:.1rem 0 0; }
table { border-collapse:collapse; width:100%; font-variant-numeric:tabular-nums; }
th, td { text-align:left; padding:.35rem .6rem .35rem 0; border-bottom:1px solid var(--line); vertical-align:top; }
th { color:var(--muted); font-weight:500; white-space:nowrap; }
td.num { text-align:right; padding-right:1.2rem; white-space:nowrap; }
.card { background:var(--card); border:1px solid var(--line); border-radius:6px; padding:.9rem 1rem; margin:.8rem 0; }
.state-healthy { color:var(--ok); } .state-failed { color:var(--bad); } .state-starting { color:var(--wait); }
pre { background:var(--bg); border:1px solid var(--line); border-radius:4px; padding:.7rem .8rem; overflow-x:auto; margin:.6rem 0 0; font-size:12.5px; }
ul.panes { list-style:none; padding:0; margin:.6rem 0 0; }
ul.panes li { padding:.55rem 0; border-bottom:1px solid var(--line); }
ul.panes a { font-weight:600; color:var(--fg); }
ul.panes .why { color:var(--muted); }
.muted { color:var(--muted); }
.undetermined { color:var(--muted); font-style:italic; }
form { margin:.4rem 0; display:flex; gap:.7rem; align-items:baseline; flex-wrap:wrap; }
button { font:inherit; padding:.2rem .7rem; border:1px solid var(--line); background:var(--card); color:var(--fg); border-radius:4px; cursor:pointer; }
code { font-family:ui-monospace,SFMono-Regular,Menlo,monospace; font-size:12.5px; }
iframe.pane { width:100%; height:calc(100vh - 9rem); border:1px solid var(--line); border-radius:6px; background:var(--bg); display:block; margin:.6rem 0; }
`

// layout wraps every console page. Panes that are their own application, such
// as the telemetry viewer, do not use it; this is for the pages the console
// renders itself.
var layout = template.Must(template.New("layout").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Title}} — pw dev</title><style>` + style + `</style></head>
<body><nav><span class="brand">pw dev</span>
<a href="/"{{if eq .Slug ""}} class="here"{{end}}>overview</a>
{{range .Panes}}{{if .Enabled}}<a href="/{{.Slug}}/"{{if eq $.Slug .Slug}} class="here"{{end}}>{{.Title}}</a>{{end}}{{end}}
</nav><main>{{.Body}}</main></body></html>`))

type navPane struct {
	Slug    string
	Title   string
	Enabled bool
}

type layoutData struct {
	Title string
	Slug  string
	Panes []navPane
	Body  template.HTML
}

func (c *Console) render(w http.ResponseWriter, slug, title string, body template.HTML) {
	renderPage(w, c.nav(), slug, title, body)
}

// renderPage writes one console page. Both the console's own pages and a pane
// that renders with the console layout go through here, so the nav and the
// headers cannot drift between them.
func renderPage(w http.ResponseWriter, nav []navPane, slug, title string, body template.HTML) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = layout.Execute(w, layoutData{Title: title, Slug: slug, Panes: nav, Body: body})
}

// navFrom reads the navigation the console attached to the request. A pane
// reached outside a console — in a test, say — renders with no nav rather than
// failing.
func navFrom(r *http.Request) []navPane {
	nav, _ := r.Context().Value(navKey{}).([]navPane)
	return nav
}

var indexBody = template.Must(template.New("index").Parse(`
<h1>{{.Project.Name}}</h1>
<p class="sub">environment <code>{{.Project.Environment}}</code> ·
{{if .Project.ApplicationURL}}application <a href="{{.Project.ApplicationURL}}">{{.Project.ApplicationURL}}</a>
{{else}}<span class="undetermined">application address undetermined</span>{{end}}</p>

<h2>Developer loop</h2>
<div class="card">
{{if .State.Phase}}
  <div><strong class="state-{{.State.Status}}">{{.StatusWord}}</strong> · {{.State.Phase}}
  {{if .Ago}}<span class="muted"> · {{.Ago}}</span>{{end}}</div>
  {{with .State.Diagnostic}}<pre>{{.Text}}</pre>{{end}}
{{else}}
  <span class="muted">no phase reported yet</span>
{{end}}
</div>

<h2>API reference</h2>
<div class="card">
{{if .Project.APIDocURL}}<a href="{{.Project.APIDocURL}}">{{.Project.APIDocURL}}</a>
<div class="muted">served by the application, so it answers while the application is running</div>
{{else}}<span class="muted">the application serves no API documentation UI</span>
{{if .Project.APIDocKey}}<div class="why muted">enable with <code>{{.Project.APIDocKey}}</code> in the runtime configuration</div>{{end}}
{{end}}
</div>

{{if .Error}}<p class="state-failed">{{.Error}}</p>{{end}}
{{if .Seeded}}<p class="state-healthy">the seed datasets were applied</p>{{end}}
{{if .CanReseed}}
<h2>Development data</h2>
<form method="post" action="/api/reseed">
<button>reseed</button>
<span class="muted">applies the project's seed datasets. Seeding is clear-insert, so it empties the tables they target — which is what makes it the way back from an editing session.</span>
</form>
{{end}}

<h2>Panes</h2>
<ul class="panes">
{{range .Panes}}<li>
{{if .Enabled}}<a href="/{{.Slug}}/">{{.Title}}</a>{{else}}<span class="muted">{{.Title}}</span>{{end}}
<div class="muted">{{.Summary}}</div>
{{if not .Enabled}}<div class="why">disabled · enable with <code>{{.DisabledBy}}</code></div>{{end}}
</li>{{end}}
</ul>
`))

type indexData struct {
	Project    Project
	State      State
	StatusWord string
	Ago        string
	Panes      []Pane
	CanReseed  bool
	Seeded     bool
	Error      string
}

func (c *Console) index(w http.ResponseWriter, r *http.Request) {
	state := c.state.get()
	data := indexData{
		Project:    c.project,
		State:      state,
		StatusWord: statusWord(state.Status),
		Panes:      c.panes,
		CanReseed:  c.CanReseed(),
		Seeded:     r.URL.Query().Get("seeded") != "",
		Error:      r.URL.Query().Get("error"),
	}
	if !state.Since.IsZero() {
		data.Ago = since(state.Since)
	}
	c.render(w, "", "overview", buildHTML(indexBody, data))
}

type framedData struct {
	Slug    string
	Title   string
	Summary string
	Entry   string
}

// framePage is the page a foreign pane is shown inside.
//
// The frame carries the console navigation and nothing else, so the pane keeps
// its own document and its own stylesheet. The source is the same path the pane
// serves, one segment deeper, which is why the pane's own links keep working
// inside it.
var framePage = template.Must(template.New("frame").Parse(`
<p class="sub">{{.Summary}}</p>
<iframe class="pane" src="{{.Entry}}" title="{{.Title}}"></iframe>
<p class="sub"><a href="{{.Entry}}">open it on its own</a></p>
`))

func statusWord(status Status) string {
	switch status {
	case StatusHealthy:
		return "running"
	case StatusFailed:
		return "failed"
	case StatusStarting:
		return "starting"
	}
	return "unknown"
}

// since is a coarse age. The index is read to answer "is this current", which a
// rounded number answers and a precise one only makes harder to scan.
func since(at time.Time) string {
	seconds := int(time.Since(at).Seconds())
	switch {
	case seconds < 5:
		return "just now"
	case seconds < 90:
		return itoa(seconds) + "s ago"
	case seconds < 5400:
		return itoa(seconds/60) + "m ago"
	default:
		return itoa(seconds/3600) + "h ago"
	}
}

func buildHTML(tmpl *template.Template, data any) template.HTML {
	var out strings.Builder
	if err := tmpl.Execute(&out, data); err != nil {
		// The templates are constants over data this package builds, so this
		// is a bug in the console rather than anything the project did. Saying
		// what failed beats a page that only says something broke.
		return template.HTML("<p class=\"state-failed\">console template error: " +
			template.HTMLEscapeString(err.Error()) + "</p>")
	}
	return template.HTML(out.String())
}
