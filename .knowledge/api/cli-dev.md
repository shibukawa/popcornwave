---
id: api:cli-dev
type: api
title: petitweb dev
---
petitweb dev regenerates the configured server package and runs it with host Go for rapid local development.

```yaml
usage: "petitweb dev [--addr :8080]"
steps:
  - run api:cli-generate
  - execute "go run" for data:project-config dev package
environment:
  ADDR: flag or configured address
process:
  - inherit stdin, stdout, and stderr
  - forward interrupt and termination signals
  - return child exit code
mvp_exclusion: automatic file watching
```
