package pwstory

import "html/template"

// The storybook is served by the harness rather than by the console, so it
// carries its own chrome. The style is kept close to the console's on purpose:
// the developer moves between them and a second visual language would be one
// more thing to learn for no gain.
const style = `
:root { color-scheme: light dark; --bg:#fff; --fg:#1a1a1a; --muted:#666; --line:#e3e3e3; --card:#fafafa; --bad:#b3261e; }
@media (prefers-color-scheme: dark) {
  :root { --bg:#141414; --fg:#e8e8e8; --muted:#9a9a9a; --line:#2c2c2c; --card:#1c1c1c; --bad:#f2b8b5; }
}
* { box-sizing: border-box; }
body { margin:0; background:var(--bg); color:var(--fg); font:14px/1.6 ui-sans-serif,system-ui,-apple-system,"Segoe UI",sans-serif; }
.layout { display:flex; min-height:100vh; }
aside { width:16rem; border-right:1px solid var(--line); padding:1rem; flex:none; overflow:auto; }
main { flex:1; padding:1.25rem 1.5rem 4rem; min-width:0; }
h1 { font-size:1.25rem; margin:0 0 .2rem; }
h2 { font-size:.95rem; margin:1.6rem 0 .5rem; font-weight:600; }
p.sub { color:var(--muted); margin:0 0 1rem; }
aside .group { color:var(--muted); font-size:12px; text-transform:uppercase; letter-spacing:.05em; margin:1rem 0 .3rem; }
aside a { display:block; padding:.2rem 0; color:var(--fg); text-decoration:none; }
aside a:hover { text-decoration:underline; }
aside a.here { font-weight:700; }
.tag { color:var(--muted); font-size:11px; margin-left:.35rem; }
.preview { border:1px solid var(--line); border-radius:6px; background:var(--card); overflow:hidden; }
.preview iframe { width:100%; height:26rem; border:0; background:#fff; display:block; }
pre { background:var(--card); border:1px solid var(--line); border-radius:6px; padding:.8rem; overflow-x:auto; font-size:12.5px; margin:0; }
.fail { border-color:var(--bad); color:var(--bad); }
.toggles a { color:var(--muted); text-decoration:none; margin-right:.8rem; font-size:13px; }
.toggles a.on { color:var(--fg); font-weight:600; }
code { font-family:ui-monospace,SFMono-Regular,Menlo,monospace; font-size:12.5px; }
a.back { display:block; color:var(--muted); text-decoration:none; font-size:12px; margin-bottom:.6rem; }
a.back:hover { color:var(--fg); }
`

const shellTemplate = `<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{template "title" .}} — pw dev</title><style>` + style + `</style></head>
<body><div class="layout">
<aside>
{{if .Prefix}}<a class="back" href="/">&larr; pw dev console</a>{{end}}
<strong>storybook</strong>
{{$current := .Template}}
{{range $index, $t := .Templates}}
{{if or (eq $index 0) (ne $t.Package (index $.Templates (add $index -1)).Package)}}<div class="group">{{$t.Package}}</div>{{end}}
<a href="{{$.Prefix}}/story/{{$t.Package}}/{{$t.Name}}"{{if and $current (eq $current.Name $t.Name) (eq $current.Package $t.Package)}} class="here"{{end}}>{{$t.Name}}{{if not $t.Exported}}<span class="tag">unexported</span>{{end}}</a>
{{end}}
</aside>
<main>{{template "body" .}}</main>
</div></body></html>`

var functions = template.FuncMap{
	"add": func(a, b int) int { return a + b },
}

var indexPage = template.Must(template.New("index").Funcs(functions).Parse(
	`{{define "title"}}storybook{{end}}
{{define "body"}}
<h1>Templates</h1>
<p class="sub">{{len .Templates}} generated from this project, rendered on their own.</p>
{{if not .Templates}}<p class="sub">Nothing is registered. A project with no template tree generates none.</p>{{end}}
<h2>What a story is</h2>
<p class="sub">Every template is bound to parameters synthesized from its own type, so a
story is the markup the template produces and not the data an application would have
given it. The same story renders identically every time.</p>
{{end}}` + shellTemplate))

var storyPage = template.Must(template.New("story").Funcs(functions).Parse(
	`{{define "title"}}{{.Template.Name}}{{end}}
{{define "body"}}
{{$r := .Rendering}}
<h1>{{.Template.Name}}</h1>
<p class="sub"><code>{{.Template.Package}}</code>{{if not .Template.Exported}} · unexported, reachable only from inside its own package{{end}}</p>

{{if $r.HasShell}}
<div class="toggles">
<a href="{{$.Prefix}}/story/{{.Template.Package}}/{{.Template.Name}}" class="{{if not $r.InShell}}on{{end}}">on its own</a>
<a href="{{$.Prefix}}/story/{{.Template.Package}}/{{.Template.Name}}?shell=1" class="{{if $r.InShell}}on{{end}}">inside the document shell</a>
</div>
{{end}}

{{if $r.Failed}}
<h2>Failed to render</h2>
<pre class="fail">{{$r.Failed}}</pre>
{{else}}
<h2>Rendered</h2>
<div class="preview">
<iframe src="{{$.Prefix}}/raw/{{.Template.Package}}/{{.Template.Name}}{{if $r.InShell}}?shell=1{{end}}" title="{{.Template.Name}}"></iframe>
</div>
<h2>HTML</h2>
<pre>{{$r.Source}}</pre>
{{end}}

<h2>Parameters</h2>
<pre>{{$r.Params}}</pre>
{{end}}` + shellTemplate))
