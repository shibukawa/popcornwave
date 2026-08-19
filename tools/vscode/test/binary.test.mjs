// The resolution order policy:editor-tool-execution fixes, and the two things
// it forbids: resolving by running something, and reaching the network. The
// second is structural — nothing here can — and the first is what makes the
// injected predicate below the whole test surface.

import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { test } from "node:test";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
const here = dirname(fileURLToPath(import.meta.url));
const { resolvePw, DEVBOX_PROFILE } = require(join(here, "..", "src", "binary.js"));

/** Resolves against a fixed set of executables rather than a real disk. */
const withExecutables = (...paths) => {
  const present = new Set(paths);
  return (candidate) => present.has(candidate);
};

test("the workspace devbox environment wins over PATH", async () => {
  const devbox = join("/work", DEVBOX_PROFILE, "pw");
  const resolved = resolvePw({
    folders: ["/work"],
    env: { PATH: "/usr/local/bin" },
    platform: "linux",
    isExecutable: withExecutables(devbox, "/usr/local/bin/pw"),
  });

  assert.equal(resolved.path, devbox);
  assert.match(resolved.source, /devbox/);
});

test("PATH is searched in order when the workspace has no devbox profile", async () => {
  const resolved = resolvePw({
    folders: ["/work"],
    env: { PATH: "/first:/second" },
    platform: "linux",
    isExecutable: withExecutables("/second/pw"),
  });

  assert.equal(resolved.path, "/second/pw");
  assert.equal(resolved.source, "PATH");
});

test("the configured path is the last resort, not the first", async () => {
  // The order is the policy's. A setting that overrode a resolved workspace
  // binary would let an editor run a different pw from the one the project's
  // own commands run, which is the disagreement stage 2 exists to avoid.
  const resolved = resolvePw({
    folders: [],
    env: { PATH: "/usr/bin" },
    configured: "/opt/pw/pw",
    platform: "linux",
    isExecutable: withExecutables("/usr/bin/pw", "/opt/pw/pw"),
  });

  assert.equal(resolved.path, "/usr/bin/pw");
});

test("the configured path is used when nothing else resolves", async () => {
  const resolved = resolvePw({
    folders: [],
    env: { PATH: "/usr/bin" },
    configured: "/opt/pw/pw",
    platform: "linux",
    isExecutable: withExecutables("/opt/pw/pw"),
  });

  assert.equal(resolved.path, "/opt/pw/pw");
  assert.equal(resolved.source, "popcornweb.pw.path");
});

test("a relative configured path is refused rather than guessed at", async () => {
  const resolved = resolvePw({
    folders: [],
    env: {},
    configured: "bin/pw",
    platform: "linux",
    isExecutable: () => true,
  });

  assert.equal(resolved.path, null);
  assert.match(resolved.reason, /absolute/);
});

test("a configured path that is not executable says so", async () => {
  const resolved = resolvePw({
    folders: [],
    env: {},
    configured: "/opt/pw/pw",
    platform: "linux",
    isExecutable: () => false,
  });

  assert.equal(resolved.path, null);
  assert.match(resolved.reason, /not an executable file/);
});

test("nothing found names the install instruction and what still works", async () => {
  const resolved = resolvePw({
    folders: ["/work"],
    env: { PATH: "/usr/bin" },
    platform: "linux",
    isExecutable: () => false,
  });

  assert.equal(resolved.path, null);
  assert.match(resolved.reason, /go install github\.com\/shibukawa\/popcornweb\/cmd\/pw@latest/);
  assert.match(resolved.reason, /Highlighting and formatting keep working/);
});

test("Windows looks for pw.exe", async () => {
  const resolved = resolvePw({
    folders: [],
    env: { Path: "C:\\tools" },
    platform: "win32",
    isExecutable: withExecutables(join("C:\\tools", "pw.exe")),
  });

  assert.equal(resolved.path, join("C:\\tools", "pw.exe"));
});

test("an empty PATH entry is skipped rather than resolved against the cwd", async () => {
  // An empty entry means the current directory, and resolving a binary from
  // whatever directory the extension host happens to have is exactly the
  // workspace-controlled input workspace trust exists to gate.
  const resolved = resolvePw({
    folders: [],
    env: { PATH: ":/usr/bin" },
    platform: "linux",
    isExecutable: (candidate) => candidate === "pw" || candidate === "/usr/bin/pw",
  });

  assert.equal(resolved.path, "/usr/bin/pw");
});
