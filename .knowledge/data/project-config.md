---
id: data:project-config
type: data
title: Project Configuration
---
petitweb.yaml stores explicit project defaults consumed by every Petitweb command.

```yaml
schema:
  version: 1
  module: example.com/myapp
  packages:
    - ./cmd/server
  build:
    output: dist/server
    target: ""
  dev:
    package: ./cmd/server
    address: ":8080"
  openapi:
    enabled: true
  templates:
    - package: ./internal/views
      output: petitweb_template_gen.go
      entries:
        - name: UserPage
          source: templates/user.html.tmpl
          data: UserPageData
rules:
  - relative paths resolve from the config file directory
  - unknown keys are errors
  - command flags override config values
  - missing config is an error except for api:cli-init
  - template data types belong to the configured template package in the first release
```
