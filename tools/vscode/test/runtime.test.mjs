// Reading the dev loop's state. What matters is the not-failing case and the
// stale case: a finding that outlives the build it described sends a reader to
// code that no longer exists.

import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { test } from "node:test";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
const here = dirname(fileURLToPath(import.meta.url));
const { loopFinding, supersedes, loopStateURL } = require(join(here, "..", "src", "runtime.js"));

test("a loop that is not failing reports nothing to show", async () => {
  assert.equal(loopFinding({ phase: "serving", build: "3" }).kind, "clear");
  assert.equal(loopFinding(null).kind, "clear");
  assert.equal(loopFinding({ diagnostic: { text: "" } }).kind, "clear");
});

test("a failure carries its position and the phase that produced it", async () => {
  const finding = loopFinding({
    phase: "generate",
    build: "4",
    diagnostic: { text: "unknown external Foo", file: "templates/card.pw.html", line: 12, column: 3 },
  });

  assert.equal(finding.kind, "finding");
  assert.equal(finding.file, "templates/card.pw.html");
  assert.equal(finding.line, 12);
  assert.match(finding.message, /^generate: /);
});

test("a zero position means no position, not line zero", async () => {
  // The record says so explicitly, and a diagnostic placed on line zero would
  // be a position the loop never claimed.
  const finding = loopFinding({ phase: "build", diagnostic: { text: "boom", file: "", line: 0, column: 0 } });

  assert.equal(finding.file, null);
  assert.equal(finding.line, 1);
});

test("a finding from a new build replaces the old one", async () => {
  const older = { build: "1", message: "generate: boom" };

  assert.ok(supersedes(older, { build: "2", message: "generate: boom" }));
  assert.ok(supersedes(older, { build: "1", message: "generate: something else" }));
  assert.ok(!supersedes(older, { build: "1", message: "generate: boom" }));
  assert.ok(supersedes(null, { build: "1", message: "x" }));
});

test("the loop state is read from the console's own endpoint", async () => {
  assert.equal(loopStateURL("http://localhost:18081"), "http://localhost:18081/api/loop-state");
  assert.equal(loopStateURL("http://localhost:18081/"), "http://localhost:18081/api/loop-state");
  assert.equal(loopStateURL(""), null);
  assert.equal(loopStateURL(undefined), null);
});
