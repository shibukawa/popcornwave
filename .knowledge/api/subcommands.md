---
id: api:subcommands
type: api
title: Application Subcommands
---
Applications may register typed subcommand inputs but retain explicit dispatch control outside the HTTP lifecycle.

```yaml
surface:
  - SubCommand[T](name, help string)
  - Command[T]() returns parsed command state
flow:
  - register every command input type during initialization
  - call api:runtime-configuration ParseConfig
  - inspect Command[T] for the selected command
  - application dispatches command logic or starts api:application-lifecycle
rules:
  - name is the stable CLI token and help is its human-readable description
  - name and help are compile-time strings consumed by system:tinybind generation
  - Run does not dispatch application subcommands
  - subcommand fields participate in the same generated config and CLI parsing system
  - exact Command lookup return shape may be refined without changing explicit application dispatch
```
