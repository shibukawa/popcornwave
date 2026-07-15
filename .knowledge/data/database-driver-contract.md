---
id: data:database-driver-contract
type: data
title: Database Driver Contract
---
The contrib database contract is the minimal database/sql/driver surface required for useful TinyGo web services.

```yaml
required_interfaces:
  - driver.DriverContext
  - driver.Connector
  - driver.Conn
  - driver.ConnPrepareContext
  - driver.ExecerContext
  - driver.QueryerContext
  - driver.ConnBeginTx
  - driver.Pinger
  - driver.Rows
  - driver.Stmt
  - driver.Tx
value_types:
  - nil
  - int64
  - float64
  - bool
  - string
  - []byte
  - time.Time
behavior:
  - context deadline cancels or closes blocked operations
  - rows and statements are idempotently closable
  - protocol errors poison unusable connections
  - server errors retain stable code and safe message
  - placeholders follow each database native convention
tests:
  - database/sql integration suite
  - leak and cancellation tests
  - live server interoperability matrix
```
