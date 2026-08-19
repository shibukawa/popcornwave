// What the client tells the server to be, checked without an extension host.

import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { test } from "node:test";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
const here = dirname(fileURLToPath(import.meta.url));
const { LANGUAGES, WATCHED_FILES, serverInvocation, documentSelector } = require(
  join(here, "..", "src", "client.js"),
);

test("the server is started as pw lsp on stdio", async () => {
  const { command, args } = serverInvocation("/usr/bin/pw");

  assert.equal(command, "/usr/bin/pw");
  assert.deepEqual(args, ["lsp", "--stdio"]);
});

test("the workspace root and the log file are passed when they are set", async () => {
  const { args } = serverInvocation("/usr/bin/pw", { root: "/work", log: "/tmp/pw.log" });

  assert.deepEqual(args, ["lsp", "--stdio", "--root=/work", "--log=/tmp/pw.log"]);
});

test("no log argument is passed by default", async () => {
  // The server writes nothing to the workspace unless it is asked to, which is
  // what lets it run in a workspace the developer only opened to read.
  const { args } = serverInvocation("/usr/bin/pw", { root: "/work" });

  assert.ok(!args.some((argument) => argument.startsWith("--log")));
});

test("the selector covers the three dialects and only real files", async () => {
  const selector = documentSelector();

  assert.deepEqual(
    selector.map((entry) => entry.language).sort(),
    ["pw-dynamo", "pw-html", "pw-sql"],
  );
  assert.ok(selector.every((entry) => entry.scheme === "file"));
  assert.equal(selector.length, LANGUAGES.length);
});

test("the project configuration is watched, so the server reloads without a restart", async () => {
  // The server registers no watcher of its own; the editor owns file events,
  // and this is the glob it is asked for.
  assert.equal(WATCHED_FILES, "**/popcornweb.toml");
});
