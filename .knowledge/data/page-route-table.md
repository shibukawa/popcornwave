---
id: data:page-route-table
type: data
title: Page Route Table
---
The generated registry publishes what the filesystem knows about every page route and action endpoint, and nothing more, so a sitemap or a route inspector needs no metadata format.

```yaml
location: api:page-registry
not_to_be_confused_with:
  data:route-table: the build-time analysis result covering every route in the application, which tooling reads; this one is a runtime value a page tree publishes about itself
routes:
  entry: Pattern, Path, Dir, and Params
  example: 'Pattern "GET /users/{id}", Path "/users/{id}", Dir "users/id_", Params ["id"]'
  source: the concept:page-tree walk, so the pattern, the method, and which segments are dynamic all come from directory names
actions:
  entry: Pattern, Path, Dir, Handler, and Hash
  example: 'Pattern "POST /_action/00369cf962b6/Rename", Dir "users/id_", Handler "Rename"'
  purpose: making the api:page-action-endpoint surface inspectable rather than implicit
excluded:
  expanded_values: which values a dynamic segment actually takes is application data and stays with the application
  openapi: neither table feeds a document, per decision:dual-router-coexistence
consumers:
  planned: a sitemap, a robots policy, and a route listing built by the framework or the application from these tables
  none_yet: the framework holds no opinion about page metadata in this rung
```
