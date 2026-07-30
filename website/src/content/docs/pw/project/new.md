---
title: pw new
description: Scaffold one more handler, its route, and its template.
sidebar:
  order: 3
---

```sh
pw new [handler]
```

A handler needs three things that have to agree with each other: a literal route
pattern, a registration on the package mux, and — for a page — a template whose
component name matches what the handler renders. `pw new` writes them together.

Like [`pw add`](/pw/project/add/), it runs inside an existing project, asks in a
wizard, and writes only after the review screen is accepted.

`handler` is the only implemented kind. The command is shaped as `pw new <kind>`
so a second one costs an entry, a step list, and its scaffold branch.

## The questions

| Step | Answer |
| --- | --- |
| Package | where the handler goes; only `generate.handlers` directories are offered |
| Method | `GET`, `POST`, `PUT`, `PATCH`, or `DELETE` |
| Path | a Go 1.22 pattern: `/tasks`, `/tasks/{id}`, or `/assets/` for a subtree |
| Name | the function and file stem; blank keeps the one derived from the route |
| Response | an HTML page, or a JSON answer through `pw.WriteAPI` |
| Request input | whether to scaffold a typed input and the `pw.Parse` call that fills it |

**The package defaults to your working directory** when it lies inside the
handler purpose, because that is where you already are. A directory outside
`generate.handlers` is never offered: a route there is analyzed by nothing, so
it would be missing from the generated OpenAPI. An HTML response additionally
requires the directory to be under `generate.templates`, since the page template
is read by a different purpose — see
[`pw generate`](/pw/project/generate/#what-it-reads).

## What it writes

```
  create  handlers/getTasks_handler.go
  create  handlers/getTasks.pw.html
```

The handler registers its route in `init()`, which is what route discovery
reads:

```go
func init() { mux.HandleFunc("GET /tasks", getTasks) }

func getTasks(w http.ResponseWriter, r *http.Request) {
	pw.WriteHTML(w, r, GetTasks(GetTasksParams{Name: "World"}))
}
```

A package that has no mux yet also gets its `index.go`. Mounting that package in
`main.go` is the application's call, so it is printed as a manual step rather
than injected.

[`pw generate`](/pw/project/generate/) runs afterwards, so the `_pw_gen.go`
artifacts exist before the next build.

## Refusals

A duplicate route is caught before anything is written, by reading the literal
patterns the target package already registers:

```
pw: GET /tasks is already registered in handlers/tasks_handler.go
```

An existing destination file is a conflict, never an overwrite. Existing handler
sources, `main.go`, and the document shell are never edited.

If only the generation step fails, the written sources stay: they are
handwritten Go and `.pw.html` that you own and fix.

## Exit status

| Situation | Exit |
| --- | --- |
| handler written | 0 |
| wizard canceled | 0, nothing written |
| no terminal | non-zero with usage |
| duplicate route, conflict, or invalid pattern | non-zero with the path and the reason |
