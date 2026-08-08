---
id: requirement:engine-parameterized-docs
type: requirement
title: Engine-Parameterized Documentation
---
Where the framework's own output differs by requirement:database-engine-selection engine, the documentation shows every engine in synchronized tabs, so a reader on Postgres is not reading a SQLite tutorial and translating as they go.

```yaml
audience: actor:application-developer
first_application: the database tutorial chapter, per requirement:tutorial-capability-growth
what_varies:
  migration: the data:migration-source starter schema, whose DDL api:cli-init and api:cli-add write in the dialect of the selected engine
  query: the .pw.sql example and the placeholder syntax flow:sql-generation compiles it to
  configuration: the data:middleware-runtime-config rdb DSN, which rule:rdb-dsn-resolution resolves per engine
  driver_import: the selected engine's blank import; every engine is application-linked, including sqlite
  development_server: the Devbox package and the server the reader has to start, which sqlite does not have at all
  reason_it_varies: decision:server-sql-support-tier translates nothing between engines, so a chapter written in one dialect is wrong in the other two
presentation:
  form: tabs, one per engine
  engines: sqlite, postgres, and mysql, in that order, with sqlite selected
  synchronized: every tab group on a page switches together, so choosing an engine once carries down the page
  default_reason: sqlite is the api:cli-init default and needs no server, which is what makes it the one a reader can follow without deciding anything first
  outside_tabs: prose that holds for every engine stays prose; a tab group that repeats identical content is worse than no tab group
mechanism:
  component: the tab component ships with the site's documentation framework and is available to MDX pages only
  conversion: a chapter using tabs becomes .mdx
  synchronized: one syncKey per page, so every group on it switches together
  base_links:
    feared: that leaving the Markdown processor would drop the rewriting that lets a page link with a plain root-absolute path, breaking every internal link under the deployed base
    measured: it does not; a converted chapter builds with the same based links as before and none unbased, checked on the built HTML of both language versions
    conclusion: an MDX chapter costs nothing here, so the tab component is available wherever this requirement applies
acceptance:
  - a reader who selects an engine sees that engine's DDL, query, DSN, and imports for the rest of the page
  - every tab of a group is complete on its own, with nothing that only the sqlite tab explains
  - a converted page's internal links resolve under the deployed base, verified on the built HTML rather than assumed
  - the language versions carry the same engines in the same order
non_goals:
  - an engine tab for anything the framework writes identically, such as handler code
  - engines outside decision:server-sql-support-tier
  - a per-engine variant of api:dynamo-package content, which is not a SQL engine and shares no example with one
```
