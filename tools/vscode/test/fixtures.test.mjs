// The drift guard of rule:template-grammar-scopes.
//
// Every .pw.html, .pw.sql, and .pw.dynamo source in this repository is
// tokenized and compared against a committed snapshot. A grammar edit that
// changes what any real source looks like has to show up as a reviewed diff,
// and an upstream syntax change that the grammar has not caught up with shows
// up the same way.
//
// Refresh with: UPDATE_SNAPSHOT=1 npm test

import assert from "node:assert/strict";
import { readdir, readFile, writeFile } from "node:fs/promises";
import { test } from "node:test";
import { dirname, join, relative, sep } from "node:path";
import { fileURLToPath } from "node:url";

import { SCOPE_BY_EXTENSION, SKIP_DIRECTORIES, tokenize } from "./tokenize.mjs";

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(here, "..", "..", "..");
const snapshotPath = join(here, "snapshots", "tokens.txt");

async function* walk(directory) {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    if (entry.isDirectory()) {
      if (SKIP_DIRECTORIES.has(entry.name)) {
        continue;
      }
      yield* walk(join(directory, entry.name));
    } else if (entry.isFile()) {
      yield join(directory, entry.name);
    }
  }
}

function scopeFor(path) {
  for (const [extension, scope] of Object.entries(SCOPE_BY_EXTENSION)) {
    if (path.endsWith(extension)) {
      return scope;
    }
  }
  return null;
}

async function collect() {
  const sources = [];
  for await (const path of walk(repoRoot)) {
    const scope = scopeFor(path);
    if (scope) {
      sources.push({ path, scope });
    }
  }
  sources.sort((a, b) => a.path.localeCompare(b.path));

  const lines = [];
  for (const { path, scope } of sources) {
    const name = relative(repoRoot, path).split(sep).join("/");
    lines.push(`## ${name} [${scope}]`);
    const tokens = await tokenize(await readFile(path, "utf8"), scope);
    for (const token of tokens) {
      // Drop the root scope; every token carries it and it adds only noise.
      const scopes = token.scopes.filter((s) => s !== scope);
      lines.push(`${token.line}\t${JSON.stringify(token.text)}\t${scopes.join(" ")}`);
    }
    lines.push("");
  }
  return { sources, text: lines.join("\n") };
}

test("every repository source tokenizes as the snapshot records", async () => {
  const { sources, text } = await collect();

  assert.ok(sources.length > 0, "no .pw.* source was discovered");
  for (const scope of Object.values(SCOPE_BY_EXTENSION)) {
    assert.ok(
      sources.some((s) => s.scope === scope),
      `no fixture exercises ${scope}`,
    );
  }

  if (process.env.UPDATE_SNAPSHOT) {
    await writeFile(snapshotPath, text);
    return;
  }

  let expected;
  try {
    expected = await readFile(snapshotPath, "utf8");
  } catch {
    assert.fail(
      `no snapshot at ${snapshotPath}; create it with UPDATE_SNAPSHOT=1 npm test`,
    );
  }

  if (expected !== text) {
    const want = expected.split("\n");
    const got = text.split("\n");
    const at = want.findIndex((line, index) => line !== got[index]);
    assert.fail(
      [
        "tokenization drifted from the committed snapshot.",
        `first difference at snapshot line ${at + 1}:`,
        `  expected: ${want[at] ?? "<end of file>"}`,
        `  actual:   ${got[at] ?? "<end of file>"}`,
        "If the change is intended, refresh with UPDATE_SNAPSHOT=1 npm test and review the diff.",
      ].join("\n"),
    );
  }
});
