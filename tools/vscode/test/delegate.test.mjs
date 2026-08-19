// The delegated formatting path. What matters is the probe: it decides whether
// the resolved pw can be trusted with a buffer, and it is a property check
// rather than a version check.

import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { test } from "node:test";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
const here = dirname(fileURLToPath(import.meta.url));
const extensionRoot = join(here, "..");
const { DelegatedFormatter, PROBE_SOURCE, formatArgs, formatFailure } = require(
  join(extensionRoot, "src", "delegate.js"),
);
const { FormatError } = require(join(extensionRoot, "src", "formatter.js"));

/** A pw that formats by returning what it is told to, and records its calls. */
function fakePw(reply) {
  const calls = [];
  const run = async (args, input) => {
    calls.push({ args, input });
    return reply(args, input, calls.length);
  };
  return { run, calls };
}

const ok = (stdout) => ({ code: 0, stdout, stderr: "" });

test("one buffer is formatted by its dialect on stdin", async () => {
  // pw fmt --stdin formats one stream and refuses a path, so the dialect is
  // the whole of the invocation.
  assert.deepEqual(formatArgs("sql"), ["fmt", "--stdin=sql"]);
});

test("a pw whose second pass agrees with its first is usable", async () => {
  const { run, calls } = fakePw(() => ok("formatted\n"));
  const delegated = new DelegatedFormatter(run);

  const { usable } = await delegated.usable();

  assert.equal(usable, true);
  assert.equal(calls.length, 2, "the probe has to format twice to check anything");
  assert.equal(calls[0].input, PROBE_SOURCE);
  assert.equal(calls[1].input, "formatted\n", "the second pass reads the first one's output");
});

test("a pw that formats differently on a second pass is refused", async () => {
  // The defect templatefmt fixed in v0.3.2: a literal brace run in a style
  // body gained a brace pair on every pass. The probe tests for that rather
  // than for a version number, because what the editor needs is the property.
  const { run } = fakePw((_args, _input, call) => ok(`pass ${call}\n`));

  const { usable, reason } = await new DelegatedFormatter(run).usable();

  assert.equal(usable, false);
  assert.match(reason, /second pass/);
});

test("a pw with no fmt --stdin is refused with what it said", async () => {
  const { run } = fakePw(() => ({ code: 2, stdout: "", stderr: "pw: unknown command \"fmt\"" }));

  const { usable, reason } = await new DelegatedFormatter(run).usable();

  assert.equal(usable, false);
  assert.match(reason, /unavailable \(exit 2\)/);
  assert.match(reason, /unknown command/);
});

test("a pw that cannot be run at all is refused rather than throwing", async () => {
  const delegated = new DelegatedFormatter(async () => {
    throw new Error("spawn ENOENT");
  });

  const { usable, reason } = await delegated.usable();

  assert.equal(usable, false);
  assert.match(reason, /could not be run: spawn ENOENT/);
});

test("the probe runs once per session, not once per format", async () => {
  // It is protecting a format, and running it every time would double the cost
  // of the thing it protects.
  const { run, calls } = fakePw(() => ok("x\n"));
  const delegated = new DelegatedFormatter(run);

  await delegated.usable();
  await delegated.usable();

  assert.equal(calls.length, 2);
});

test("an unchanged buffer reports no change", async () => {
  const source = "export statement F(): sql.exec {\n  DELETE FROM t\n}\n";
  const { run } = fakePw(() => ok(source));

  const result = await new DelegatedFormatter(run).format("pw-sql", source);

  assert.equal(result.changed, false);
  assert.equal(result.text, source);
});

test("a refused source becomes a FormatError naming the line", async () => {
  // The CLI writes its own prefix and the position belongs to the buffer the
  // caller sent, so both are read off rather than shown verbatim.
  const { run } = fakePw(() => ({
    code: 1,
    stdout: "",
    stderr: "pw: fmt: <stdin>:3:1: missing closing tag </p>\n",
  }));

  await assert.rejects(
    () => new DelegatedFormatter(run).format("pw-html", "broken"),
    (error) => {
      assert.ok(error instanceof FormatError);
      assert.equal(error.line, 3);
      assert.equal(error.message, "missing closing tag </p>");
      return true;
    },
  );
});

test("a failure with no position is still reported", async () => {
  const failure = formatFailure("pw: fmt: the database engine is unknown\n", 1);

  assert.equal(failure.line, null);
  assert.equal(failure.message, "the database engine is unknown");
});

test("a silent failure says which exit code it was", async () => {
  assert.match(formatFailure("", 3).message, /exited 3 and said nothing/);
});

test("an unknown language never reaches the process", async () => {
  const { run, calls } = fakePw(() => ok(""));

  await assert.rejects(
    () => new DelegatedFormatter(run).format("plaintext", "anything"),
    /no dialect for language plaintext/,
  );
  assert.deepEqual(calls, []);
});
