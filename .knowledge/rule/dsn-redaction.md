---
id: rule:dsn-redaction
type: rule
title: DSN Redaction
---
A reported DSN keeps the address and loses the credential, because hiding it whole costs the reader the question the report is opened to answer.

```yaml
applies_to:
  - policy:startup-summary, both the tree and the record
  - api:cli-doctor configuration view
  - api:cli-migrate and api:cli-seed failure messages
  - every key whose name ends in .dsn, so rdb connections, session rdb, and session redis read alike
kept:
  - scheme, which names the engine rule:rdb-dsn-resolution selects
  - host and port, which name the server this process talks to
  - the trailing path, which is the database name or the SQLite file, including :memory:
removed:
  userinfo: the credential; its presence is marked so the reader sees one was configured
  query: dropped whole, because an unrecognized parameter is where an unlisted secret would hide
form:
  with_credential: scheme://<mark>@host:port/name
  without_credential: scheme://host:port/name
  mark: the same mask configbind writes, so one idea never shows two marks
  driver_address: a go-sql-driver protocol(host:port) is unwrapped to host:port, which is how every other engine writes it
unparseable: hidden whole, because a half-read DSN is not worth the risk of printing
reason:
  - which database a process is attached to is an operational fact, not a secret
  - a summary that answers nothing is one an operator stops reading
  - the credential is the only part that grants access, so it is the only part removed
verification:
  - a DSN carrying a password produces no output containing it
  - the same DSN reads the same way in the startup summary, in doctor, and in a migrate failure
  - a sqlite path and :memory: survive unchanged, since neither can carry a credential
```
