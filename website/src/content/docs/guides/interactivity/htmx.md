---
title: Integrating htmx
description: Share one Popcorn Web component between the initial page and later fragments, while htmx replaces only the region that changed.
sidebar:
  order: 6
---

There is no server-side adapter for htmx. It needs a response shaped like the
part of the page that already exists, and Popcorn Web expresses that difference
with one call: `pw.WriteHTML` for the page, `pw.WriteHTMLFragment` for the
region.

A short response alone is not the integration. If the first render and later
swaps have separate definitions of their markup, they will drift. The useful
property is that both response paths call the same generated component.

## Load htmx

For an application that already serves its own assets, a pinned `htmx.min.js`
under `public/` is the simplest default. It is embedded in the application
binary, and no third-party origin has to enter the CSP.

```html
package templates

export component Document(children: html?): html {
<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Tasks</title>
    <script defer src="/public/htmx.min.js"></script>
  </head>
  <body><slot /></body>
</html>
}
```

A CDN works too. Pin the version in the URL and add Subresource Integrity and
`crossorigin`. The repository's `examples/htmx_fragment` works through both
choices: a pinned CDN build and the same file vendored into `public/`.

## One component, two response paths

Give the region's outer element a stable id that htmx can target.

```html
package tasks

type Task { id: string, title: string }

export component TaskList(tasks: Task[]): html {
<ul id="task-list">
  {for task in tasks}
    <li>{task.title}</li>
  {/for}
</ul>
}

export component TasksPage(query: string, tasks: Task[]): html {
<main>
  <h1>Tasks</h1>
  <form
    action="/tasks"
    method="get"
    hx-get="/tasks/list"
    hx-target="#task-list"
    hx-swap="outerHTML">
    <label for="query">Filter</label>
    <input id="query" name="q" value={query}>
    <button type="submit">Search</button>
  </form>

  <TaskList tasks={tasks} />
</main>
}
```

With JavaScript, htmx replaces the whole `#task-list` with the response from
`/tasks/list`. Without it, the browser follows the form's `action` to `/tasks`
as an ordinary navigation. `action` and `hx-get` are intentionally different
so the unenhanced route still answers with a document.

Both handlers call `TaskList`:

```go
func register(mux *pw.ServeMux) {
	mux.HandleFunc("GET /tasks", tasksPage)
	mux.HandleFunc("GET /tasks/list", taskList)
}

func tasksPage(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	matched := findTasks(query)
	pw.WriteHTML(w, r, TasksPage(TasksPageParams{
		Query: query,
		Tasks: matched,
	}))
}

func taskList(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	pw.WriteHTMLFragment(w, r, TaskList(TaskListParams{
		Tasks: findTasks(query),
	}))
}
```

The fragment body is `<ul id="task-list">…</ul>` and nothing around it: no
document shell, head, wrapper chain, or async-boundary framing. That shape
matches `hx-target="#task-list"` with `hx-swap="outerHTML"`.

One route can serve both representations by checking
`r.Header.Get("HX-Request") == "true"` in application code. The framework does
not classify the request for you. Separate routes are easier to follow first,
especially when the page path redirects but the fragment path returns markup.

## Three fragment constraints

A fragment has no document. That decides three integration rules:

| Constraint | Consequence for htmx |
| --- | --- |
| It cannot contribute to the head | the initial page loads every style and script a swapped region needs |
| It never streams | an `await` settles on the server; `hx-indicator` owns the waiting state |
| It carries no envelope around the HTML | request ordering and stale-response handling belong to htmx |

Passing a component with a head contribution to `pw.WriteHTMLFragment` answers
500. Failing visibly and leaving the old DOM in place is safer than silently
dropping the component's styles. [Fragments and islands](/guides/interactivity/fragments/)
covers the common rules plus dialog, toast, and waiting-state recipes.

## Validation status is a display decision

htmx normally swaps 2xx responses and leaves 4xx/5xx responses unswapped. A
field-validation failure that re-renders the form therefore returns that HTML
with **200**. An unreadable body, failed authorization, or missing record keeps
its real 4xx/5xx problem response.

```go
_, err := pw.Parse[createInput](r)
if err != nil {
	fields, ok := validationFields(err)
	if !ok {
		pw.WriteProblem(w, r, pw.BadRequest(err))
		return
	}
	pw.WriteHTMLFragment(w, r, TaskForm(formWithErrors(r, fields)))
	return
}
```

The 200 does not call invalid input valid. It says that the returned HTML is the
thing to display. An application can instead allow a 422 to swap in
`htmx:beforeSwap`, but that convention should be consistent across the site.

## Unsafe requests and CSRF

The template compiler inserts a CSRF hidden field into every
`<form method="post">`. htmx submits it with the rest of that form, so that path
needs no integration code.

An `hx-delete` or `hx-patch` button outside a form has no hidden field. When
`security.csrf.enabled = true`, read the default `pw_csrf` cookie at request
time and copy it to `X-CSRF-Token`. If the application changes `cookie_name` or
`header`, change the same two strings here:

```js
// public/htmx-csrf.js
const unsafe = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);

function cookie(name) {
  const prefix = `${name}=`;
  const item = document.cookie.split('; ').find((part) => part.startsWith(prefix));
  return item ? item.slice(prefix.length) : '';
}

document.addEventListener('htmx:configRequest', (event) => {
  if (!unsafe.has(event.detail.verb.toUpperCase())) return;
  const token = cookie('pw_csrf');
  if (token) event.detail.headers['X-CSRF-Token'] = token;
});
```

Load this file with `defer` after htmx. Reading the cookie just before the
request matters: a login or privilege change in another tab can rotate the
session while this page remains open. [Security](/guides/architecture/security/#how-csrf-works-here)
explains the three token channels and that rotation.

```html
<script defer src="/public/htmx.min.js"></script>
<script defer src="/public/htmx-csrf.js"></script>
```

## Sharing a page with React islands

htmx and React can share a page, but they must not manage the same children.
Point an htmx target outside a React root, or replace an element that contains
the entire root. The latter must unmount the old root and mount the inserted
one. [Integrating React](/guides/interactivity/react/) puts those operations in
a custom element's `connectedCallback` and `disconnectedCallback`.

htmx needs no additional Popcorn Web build support: one library file and
`pw.WriteHTMLFragment` complete the integration. A useful optional addition
would be a tiny framework-served adapter for the CSRF event above. It would
remove repetition around `hx-delete` without building htmx itself into the
framework.
