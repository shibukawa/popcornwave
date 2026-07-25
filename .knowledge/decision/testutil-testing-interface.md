---
id: decision:testutil-testing-interface
type: decision
title: testutil TestingT Interface
---
testutil accepts a minimal TestingT interface instead of *testing.T so the shipped framework package never imports testing.

```yaml
status: accepted
owner: api:test-run
surface:
  name: testutil.TestingT
  methods:
    - Helper()
    - Cleanup(func())
    - Fatalf(string, ...any)
    - Errorf(string, ...any)
rationale:
  - testutil is a normal package inside the framework module, not a _test.go file
  - importing testing from a shipped package pulls testing initialization into ordinary builds
  - concept:public-package-boundaries keeps framework packages free of test-harness machinery
  - the interface admits *testing.B, *testing.F, and application-owned T wrappers
  - testutil can drive its own tests with a fake T
rules:
  - add a method only when an api:test-run or api:test-seed behavior requires it
  - Fatalf marks setup failure that invalidates the test
  - Errorf marks an assertion failure that lets the test continue
  - never widen to Context, Log, Skip, Run, or Deadline without a recorded need
  - *testing.T satisfies the interface, so widening it is source compatible for standard callers
consequences:
  - system:dbtestify assertdb is unusable because it binds *testing.T and t.Context()
  - api:test-seed builds its own seeding and assertion helpers on the dbtestify core package
  - context comes from the caller or context.Background, never from a testing method
```
