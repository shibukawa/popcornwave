---
id: concept:application-entry-point
type: concept
title: Application Entry Point
---
The generated application entry point selects drivers explicitly and starts the handler package through the root pw lifecycle.

```yaml
handwritten_shape:
  imports:
    - context
    - application handlers package
    - github.com/shibukawa/popcornweb/pw
  run: pw.Run(context.Background(), handlers.Handlers())
rules:
  - data:project-config project.main selects this package for api:cli-build
  - database startup follows decision:config-driven-database
  - requirement:built-in-config-generation needs no application command type or dispatch
  - application subcommands use api:subcommands RegisterSubCommand
  - api:application-lifecycle returns startup, serving, or shutdown errors to main
  - applications needing a custom http.Server use pw.Middlewares instead
```
