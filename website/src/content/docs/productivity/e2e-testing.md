---
title: E2E Testing
description: Drive the development server with Playwright, and seed and assert the database over HTTP with the same dataset files.
sidebar:
  order: 2
---

[testutil](/productivity/testing/) starts the application you deploy and
reaches it over HTTP, but its client is Go's `http.Client`. It observes
responses and nothing after them: no script runs, no
[dialog opens](/guides/interactivity/browser-controls/), no
[fragment lands in a page](/guides/interactivity/fragments/). When the
behaviour under test includes the browser's half of the exchange, the test has
to run a browser. [Playwright](https://playwright.dev/) is that layer, and
Popcorn Wave meets it halfway: in the `pwdev` build mode the application itself
serves seed and assert endpoints, so a browser suite reads and resets the
database through the same dataset files every other test already uses.

Weigh the proportions before writing one. A browser test is the most expensive
kind this site describes: slower by an order of magnitude, and its writes are
real — no [`WithTransaction`](/productivity/testing/#withtransaction) rolls
anything back, because the test and the server are separate processes. Most
coverage belongs in `testutil`, where isolation is free. A browser test earns
its cost where the browser contributes behaviour a Go client cannot observe.

## Pointing Playwright at the development server

`npm init playwright@latest` scaffolds the suite; the part worth replacing is
the configuration. This one drives a Popcorn Wave application:

```ts
// playwright.config.ts
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
	testDir: './e2e',
	workers: 1,
	use: { baseURL: 'http://127.0.0.1:8080' },
	projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
	webServer: {
		command: 'pw dev',
		url: 'http://127.0.0.1:8080/',
		reuseExistingServer: !process.env.CI,
		timeout: 120_000,
	},
});
```

The server is [`pw dev`](/pw/project/dev/) in both places it could run. On your
machine the loop is already going, and `reuseExistingServer` lets the suite
drive that server instead of starting a rival — the same generated, migrated,
rebuilt-on-change application you are looking at. In CI the `command` starts
the identical loop: services, generation, migrations, and the development
identity provider, which is why a login page works in the suite with no
arrangement at all. The generous `timeout` covers the loop's startup, which is
a build rather than a bind.

`workers: 1` is not caution but arithmetic. Every test drives one application
holding one database, and every write is committed, so two workers would
interleave their table states. Tests inside one file already run in order;
this extends that ordering across files. A suite that outgrows serial execution
needs a database per worker, which is a heavier arrangement than most
applications ever justify.

Note what the configuration does not set: `APP_ENV`. The suite runs in the
development environment, against the development database, and that is a
decision rather than an omission — the seed endpoints below are locked to that
environment, the same
[allowlist every development relaxation uses](/guides/architecture/security/#refused-outside-development).
The consequence is worth saying plainly: **the suite reseeds the tables its
datasets name, and rows you typed in by hand do not survive a run.** If that
loss sounds expensive, the rows belonged in a
[dataset](/productivity/seed-data/).

## A first screen test

```ts
// e2e/members.spec.ts
import { test, expect } from '@playwright/test';

test('the member list shows every member', async ({ page }) => {
	await page.goto('/members');
	await expect(page.getByRole('listitem')).toHaveText(['Frank', 'Grace', 'Heidi']);
});
```

Selectors go through roles and visible text rather than CSS classes or
generated ids, so the test survives a template rewrite that keeps the page
meaning the same — the browser-suite counterpart of testing through public
routes rather than internal functions.

The three names are the rows `initial.yaml` inserts — the dataset
[Seed Data](/productivity/seed-data/) introduced. And that is the weakness of
the test as written: nothing establishes the state it asserts. The previous
run, or an afternoon of manual clicking, may have left the `member` table
holding anything. A screen test needs what a fixture gave the Go suite — a
known starting state, established by the test itself.

## Seeding from the suite

In the `pwdev` build the application answers for its own datasets:

```
POST /_pw/test/seed/{dataset}     apply a dataset to the running database
GET  /_pw/test/assert/{dataset}   compare the database against a dataset
```

Names work exactly as they do for `pw seed` and `testutil.WithSeed` —
relative to `testdata/seed`, extension optional, subdirectories allowed — and
they resolve against the same files, which is what keeps the CLI, the Go
suite, and the browser suite from drifting apart. A short helper turns the
endpoints into test vocabulary:

```ts
// e2e/db.ts
import { expect, type APIRequestContext } from '@playwright/test';

export async function seed(request: APIRequestContext, dataset: string) {
	const response = await request.post(`/_pw/test/seed/${dataset}`);
	expect(response.status(), await response.text()).toBe(204);
}

export async function assertDB(request: APIRequestContext, dataset: string) {
	const response = await request.get(`/_pw/test/assert/${dataset}`);
	expect.soft(response.status(), await response.text()).toBe(204);
}
```

Playwright's `request` fixture already carries the `baseURL`, so the calls
land on the server under test. A test that writes reseeds before it runs:

```ts
import { test, expect } from '@playwright/test';
import { seed } from './db';

test.beforeEach(async ({ request }) => seed(request, 'initial'));

test('archiving a member removes the row', async ({ page }) => {
	await page.goto('/members');
	await page.getByRole('row', { name: 'Grace' })
		.getByRole('button', { name: 'Archive' }).click();
	await expect(page.getByRole('row', { name: 'Grace' })).toHaveCount(0);
});
```

The default operation is clear-insert: each table the file names is returned
to exactly the rows it lists, and tables the file does not mention are
untouched — the same semantics as
[`server.Seed`](/productivity/testing/#reseeding-mid-test) in the Go suite,
because it is the same machinery reading the same file. Reset at the start,
never clean up at the end. A cleanup that fails poisons the *next* test,
silently; a test that begins by reseeding is correct whatever the previous one
left behind. The call is one HTTP request into the running process, so a
`beforeEach` on every test costs nothing worth measuring.

Datasets that exist only for the browser suite — a catalogue wide enough to
force pagination, say — go in a subdirectory:

```ts
await seed(request, 'e2e/wide_catalog');   // testdata/seed/e2e/wide_catalog.yaml
```

A bare `pw seed` does not descend into subdirectories, so a routine developer
reseed never applies data that only a test wanted.

Three locks decide whether the endpoints exist at all, matching every other
development relaxation: the `pwdev` build mode, `APP_ENV` resolving to `dev`,
and a caller on loopback with no forwarding header. A release binary carries no
endpoint bytes, and everything short of all three locks answers 404 from the
closed `/_pw` namespace — indistinguishable from the endpoints never having
been built.

## The fixture round trip

The Go suite's [fixture story](/productivity/testing/#fixtures) — one file as
the starting state, another as the expected state — works from the browser
suite too, because the assert endpoint is `server.AssertDB` over HTTP:

```ts
import { test, expect } from '@playwright/test';
import { seed, assertDB } from './db';

test('archiving a member archives nobody else', async ({ page, request }) => {
	await seed(request, 'initial');
	await page.goto('/members');
	await page.getByRole('row', { name: 'Grace' })
		.getByRole('button', { name: 'Archive' }).click();
	await expect(page.getByRole('row', { name: 'Grace' })).toHaveCount(0);
	await assertDB(request, 'after_archive');
});
```

The page assertion says the flow looked right; `after_archive.yaml` says the
database agrees, whole tables at a time. A handler that archives the right
member and also clears somebody else's row passes the first check and fails
the second — the same collateral damage the
[Fixtures](/productivity/testing/#fixtures) section catches in Go, caught here
without leaving TypeScript. A mismatch answers 409 with the per-table diff as
plain text, which the helper surfaces as the failure message, and `expect.soft`
mirrors `AssertDB`'s behaviour of reporting the drift without abandoning the
rest of the test.

One difference from the Go suite is worth holding on to: there is no test
transaction here, so the endpoint compares committed state. That is exactly
what a browser test produces, but it also means an assertion raced against an
in-flight request compares too early — assert after the page has shown the
result, as above. And when a check needs computation rather than comparison —
a counter, a derived value — that is a `testutil` test with
[`server.Context()`](/productivity/testing/#asserting-against-the-database),
not a browser test.

## Login flows

The [development identity provider](/productivity/dev-identity-provider/) is
part of the loop the suite already drives: `pw dev` starts it and hands the
application its issuer and client credentials, locally and in CI alike. Drive
the login page like any other page. Login *logic*, as opposed to the visible
flow, does not need a browser in the first place:
[`WithIdentityProvider`](/productivity/testing/#withidentityprovider)
completes the whole exchange inside a Go test, faster and with the database
transaction intact.

## A dedicated database

When the suite must not touch the development database — several developers
sharing one PostgreSQL, or datasets that would trample data you keep around —
the seed endpoints do not follow: they are locked to the development
environment, deliberately. Declare another environment and fall back to the
CLI, which carries no such lock:

```toml
# config.e2e.toml
[server]
port = 8090

[middleware.rdb]
enabled = true

[[middleware.rdb.connections]]
dsn = "sqlite://e2e.db"
```

`APP_ENV=e2e` then selects this file everywhere at once — set
`env: { ...process.env, APP_ENV: 'e2e' }` on the `webServer` entry and the same
loop serves this configuration, while `pw migrate` and `pw seed`
[follow the same variable](/pw/database/seed/#where-the-dsn-comes-from). The
suite seeds by running `pw seed` as a subprocess between tests instead of
posting to the endpoint — slower, since each call compiles the application to
learn the DSN, but correct. The boundary is sessions: outside `dev` the
`session.cookie.secure = false` relaxation is refused at startup, and WebKit
will not store a `Secure` cookie from plain-`http` loopback, so an application
whose flows ride the session cookie keeps its suite in the development
environment. The dedicated environment is for the rest.
