// Placing a pw doctor finding. The report names a file, a key and a file, or
// something that is neither, and a diagnostic has to land somewhere honest in
// all three cases.

import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { test } from "node:test";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";

const require = createRequire(import.meta.url);
const here = dirname(fileURLToPath(import.meta.url));
const { locate, lineOfKey, reportDiagnostics, reportLimits, SEVERITY } = require(
  join(here, "..", "src", "doctor.js"),
);

const CONFIG = [
  "[project]",
  'name = "fixture"',
  "",
  "[session]",
  'store = "cookie"',
  'secret = "x"',
  "",
  "[observability.otel]",
  "enabled = true",
  "",
].join("\n");

function fixtureRoot() {
  const root = mkdtempSync(join(tmpdir(), "pwdoctor-"));
  writeFileSync(join(root, "popcornweb.toml"), CONFIG);
  writeFileSync(join(root, "config.dev.toml"), CONFIG);
  writeFileSync(join(root, "devbox.json"), "{}");
  return root;
}

const readFixture = () => CONFIG;

test("a key names the line it is written on", async () => {
  assert.equal(lineOfKey(CONFIG, "session.secret"), 6);
  assert.equal(lineOfKey(CONFIG, "project.name"), 2);
});

test("a key that is defaulted lands on its section header, not on line 1", async () => {
  // Sending the reader to the top of the file to search for a key that is not
  // written anywhere is worse than pointing at the section it belongs to.
  assert.equal(lineOfKey(CONFIG, "session.rolling"), 4);
});

test("a dotted section header is matched as written", async () => {
  assert.equal(lineOfKey(CONFIG, "observability.otel.enabled"), 9);
});

test("an unknown key falls back to the first line rather than guessing", async () => {
  assert.equal(lineOfKey(CONFIG, "nothing.like.this"), 1);
});

test("a bare path is the file the finding is about", async () => {
  const root = fixtureRoot();
  assert.deepEqual(locate(root, "devbox.json", "config.dev.toml", readFixture), {
    file: "devbox.json",
    line: 1,
  });
});

test("a key in a file lands on that key in that file", async () => {
  const root = fixtureRoot();
  assert.deepEqual(locate(root, "session.secret in config.dev.toml", "config.dev.toml", readFixture), {
    file: "config.dev.toml",
    line: 6,
  });
});

test("evidence that names nothing lands on the environment's configuration", async () => {
  // "--online" is evidence about how the run was invoked, not about a file.
  const root = fixtureRoot();
  assert.deepEqual(locate(root, "--online", "config.dev.toml", readFixture), {
    file: "config.dev.toml",
    line: 1,
  });
});

test("a finding with no evidence at all still lands somewhere navigable", async () => {
  const root = fixtureRoot();
  assert.deepEqual(locate(root, "", "", readFixture), { file: "popcornweb.toml", line: 1 });
});

test("findings are grouped by file and named with their environment", async () => {
  // The same key is fine in dev and wrong in prod, so a message that does not
  // say which one it is about sends the reader to the wrong file.
  const root = fixtureRoot();
  const report = {
    environments: [
      {
        env: "dev",
        config_path: "config.dev.toml",
        findings: [
          { id: "PW0122", severity: "note", message: "outside the devbox shell", evidence: "devbox.json" },
          { id: "PW0201", severity: "error", message: "the session secret is weak", evidence: "session.secret in config.dev.toml" },
        ],
      },
    ],
  };

  const byFile = reportDiagnostics(root, report, readFixture);

  assert.deepEqual([...byFile.keys()].sort(), ["config.dev.toml", "devbox.json"]);
  const [session] = byFile.get("config.dev.toml");
  assert.equal(session.line, 6);
  assert.equal(session.severity, SEVERITY.error);
  assert.match(session.message, /^\[dev\] /);
});

test("what the report could not determine is carried out of it", async () => {
  // A report that looks clean because it did not look is the failure mode the
  // limits exist to prevent.
  const limits = reportLimits({
    limits: [{ Subject: "import graph", Reason: "go list failed", Effect: "the wiring checks did not run" }],
  });

  assert.equal(limits.length, 1);
  assert.match(limits[0], /import graph: go list failed — the wiring checks did not run/);
});

test("an empty report produces nothing rather than failing", async () => {
  assert.equal(reportDiagnostics(fixtureRoot(), {}, readFixture).size, 0);
  assert.deepEqual(reportLimits(undefined), []);
});
