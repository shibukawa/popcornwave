package devconsole

import (
	"context"
	"html/template"
	"net/http"
)

// TextPane shows the output of a pw subcommand, unaltered.
//
// It exists because policy:dev-console-boundary admits an action only where a
// subcommand already offers it, and the most faithful way to honour that is to
// run the command and show what it said. A second rendering of the same report
// would be a second thing to keep true, and it would disagree with the terminal
// the first time either changed.
//
// The output keeps its own layout, which is what the command spent its effort
// on: api:cli-doctor orders findings by severity and names a remedy for each,
// and reflowing that into a table would lose the ordering it means.
func TextPane(title, summary string, run func(context.Context) (string, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data := textPaneData{Title: title, Summary: summary}
		output, err := run(r.Context())
		data.Output = output
		if err != nil {
			// A command that exits nonzero has still said something worth
			// reading — api:cli-doctor exits nonzero precisely when it found
			// what it was asked to look for — so the output is shown either
			// way and the error joins it rather than replacing it.
			data.Error = err.Error()
		}
		renderPage(w, navFrom(r), title, title, buildHTML(textPanePage, data))
	})
}

type textPaneData struct {
	Title   string
	Summary string
	Output  string
	Error   string
}

var textPanePage = template.Must(template.New("textpane").Parse(`
<h1>{{.Title}}</h1>
<p class="sub">{{.Summary}}</p>
<form method="get"><button>run again</button>
<span class="muted">read only; it writes no file, starts no process, and opens no connection</span></form>
{{if .Error}}<p class="state-failed">{{.Error}}</p>{{end}}
{{if .Output}}<pre class="report">{{.Output}}</pre>
{{else}}<p class="muted">the command produced no output</p>{{end}}
`))
