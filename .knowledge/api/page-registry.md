---
id: api:page-registry
type: api
title: Page Registry
---
The generated registry in a concept:page-tree root is the one symbol an application touches to serve every discovered page and action.

```yaml
file: routes_pw_gen.go in the page tree root package
surface:
  - Register(mux, options...) installs every page route and every api:page-action-endpoint
  - Routes and Actions, the data:page-route-table values
  - Render per route package, taking the ResponseWriter and Request and composing that page inside its ancestor layouts
mux_parameter:
  form: the api:page-render-runtime router interface, named through the system:tinybind MuxType symbol, per decision:page-render-binding
  why_it_is_a_question_at_all: api:serve-mux is an alias of the standard mux outside a TinyGo build, so the two mux types only differ in the build where one of them is compiled
  omitted: the constructor function, because an empty MuxConstructor suppresses it and an interface is not something generated code can build
render_options:
  kept: the variadic render options of the built-in registry template
  precedence: api:page-render-runtime derives its options from data:html-render-config first and appends these, so a call site extends framework policy instead of replacing it
wiring:
  pages_only: concept:application-entry-point creates the mux with pw.NewServeMux, passes it to pages.Register, and hands it to pw.Run
  both_routers: the handlers package mux is created as today and pages.Register installs the page routes on it
placement:
  where: the tree root package, never beside a page
  reason: a leaf imports the root for its ancestor layouts, so a per-page composer would make the root import the leaf and Go would call it a cycle
  consequence: every generated import points down the tree
per_route_files:
  route_pw_gen.go: the route parameter struct and its decoder, in the route's own package
  page_pw_gen.go and layout_pw_gen.go: the compiled components
rules:
  - the registry is generated, so no application code lists a page route
  - a page route is registered as GET, and an action endpoint as POST
  - a decode failure answers through api:error-renderer rather than a zero value
  - Register does not create the mux, so a project keeps ownership of what else is on it
```
