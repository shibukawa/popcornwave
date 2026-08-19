// Rebuilds the formatter module and fails when the committed artifact formats
// anything differently from that fresh build, so the .wasm in the tree cannot
// drift from wasm/main.go or from the tinybind version its go.mod pins.
//
// The comparison is behavioral rather than byte for byte, because a byte
// comparison is not something a contributor can satisfy. TinyGo 0.41.1 with
// the same Go and the same Binaryen emits 610613 bytes on darwin/arm64 and
// 614645 on linux/amd64; the Go patch level moves it again, by 54 bytes
// between go1.26.0 and go1.26.5. A maintainer on a Mac could therefore never
// commit a module whose hash the Linux runner reproduces. What the extension
// promises is the output, and that is identical across all of those builds.
//
// The cost is that a source edit with no observable effect on the corpus below
// passes unnoticed. The corpus is every .pw.* source in the repository plus
// the probes here, which is what makes that a narrow gap rather than a hole.
//
// Run by CI rather than by npm test, because it needs a TinyGo toolchain that
// a contributor editing only the grammars should not have to install.

import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdtemp, readdir, readFile, rm } from "node:fs/promises";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import { dirname, join, relative, sep } from "node:path";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
const here = dirname(fileURLToPath(import.meta.url));
const extensionRoot = join(here, "..");
const repoRoot = join(extensionRoot, "..", "..");
const wasmDir = join(extensionRoot, "wasm");
const committed = join(wasmDir, "pwfmt.wasm");

const { EmbeddedFormatter, FormatError } = require(
  join(extensionRoot, "src", "formatter.js"),
);

const { SKIP_DIRECTORIES } = await import("./tokenize.mjs");

const LANGUAGE_BY_EXTENSION = {
  ".pw.html": "pw-html",
  ".pw.sql": "pw-sql",
  ".pw.dynamo": "pw-dynamo",
};

// What the repository's own sources do not exercise: the argument handling in
// wasm/main.go, the diagnostic text, and an input that is already canonical.
// A change to any of those is a change to what the extension shows a user.
const PROBES = [
  {
    name: "<probe: unformatted sql>",
    language: "pw-sql",
    source:
      "package q\ntype R{id:int}\nexport statement F(id:int):sql.one<R>{SELECT id FROM t WHERE id={id}}\n",
  },
  {
    name: "<probe: unformatted html>",
    language: "pw-html",
    source: "export component C(v: string): html {\n<p>{v}</p>\n}\n",
  },
  {
    name: "<probe: unformatted dynamo>",
    language: "pw-dynamo",
    source:
      "export statement Q(k: string): dynamo.many<R> {\ntable readings\nkey pk = {k}\n}\n",
  },
  {
    name: "<probe: canonical dynamo>",
    language: "pw-dynamo",
    source: [
      "export statement Q(k: string): dynamo.many<Reading> {",
      "  table readings",
      "  key pk = {k}",
      "}",
      "",
    ].join("\n"),
  },
  {
    name: "<probe: raw text braces>",
    language: "pw-html",
    source: [
      "export component B(label: string): html {",
      "  <head>",
      "    <style>",
      ".demo { color: crimson }",
      "</style>",
      "  </head>",
      "  <p>{label}</p>",
      "}",
      "",
    ].join("\n"),
  },
  {
    name: "<probe: syntax error>",
    language: "pw-html",
    source: "export component X(): html {\n<p>unclosed\n",
  },
  { name: "<probe: empty buffer>", language: "pw-html", source: "" },
];

const digest = async (path) =>
  createHash("sha256")
    .update(await readFile(path))
    .digest("hex");

const importsOf = async (path) =>
  WebAssembly.Module.imports(await WebAssembly.compile(await readFile(path)))
    .map((entry) => `${entry.module}.${entry.name}`)
    .sort();

/** Reduces one format to the pair of outcomes the extension can produce. */
async function outcome(formatter, language, source, name) {
  try {
    const { text, changed } = await formatter.format(language, source, name);
    return { ok: true, text, changed };
  } catch (error) {
    if (!(error instanceof FormatError)) {
      throw error;
    }
    return { ok: false, message: error.message, line: error.line };
  }
}

/** Names the first line the two results disagree on, for the report. */
function describe(before, after) {
  if (before.ok !== after.ok) {
    return before.ok
      ? `committed formatted it; the rebuild refused it (${after.message})`
      : `committed refused it (${before.message}); the rebuild formatted it`;
  }
  if (!before.ok) {
    return `diagnostic changed\n    committed: ${before.message} (line ${before.line})\n    rebuilt:   ${after.message} (line ${after.line})`;
  }
  const left = before.text.split("\n");
  const right = after.text.split("\n");
  const at = left.findIndex((line, index) => line !== right[index]);
  if (at === -1) {
    return `changed flag differs: committed ${before.changed}, rebuilt ${after.changed}`;
  }
  return `line ${at + 1} differs\n    committed: ${JSON.stringify(left[at])}\n    rebuilt:   ${JSON.stringify(right[at])}`;
}

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

const scratch = await mkdtemp(join(tmpdir(), "pwfmt-"));
const rebuilt = join(scratch, "pwfmt.wasm");

try {
  execFileSync("sh", [join(wasmDir, "build.sh"), rebuilt], { stdio: "inherit" });

  console.log(
    `committed ${(await digest(committed)).slice(0, 12)}, rebuilt ${(await digest(rebuilt)).slice(0, 12)}; ` +
      "the hashes are expected to differ across hosts",
  );

  const differences = [];

  // An import the shim does not provide fails instantiation in the extension
  // host rather than any one format, so it is checked on its own.
  const before = await importsOf(committed);
  const after = await importsOf(rebuilt);
  if (before.join() !== after.join()) {
    differences.push({
      name: "<the WASI imports the module declares>",
      detail: `committed: ${before.join(", ")}\n    rebuilt:   ${after.join(", ")}`,
    });
  }

  const committedFormatter = new EmbeddedFormatter(() => readFile(committed));
  const rebuiltFormatter = new EmbeddedFormatter(() => readFile(rebuilt));

  const cases = PROBES.slice();
  for await (const path of walk(repoRoot)) {
    const extension = Object.keys(LANGUAGE_BY_EXTENSION).find((suffix) =>
      path.endsWith(suffix),
    );
    if (!extension) {
      continue;
    }
    cases.push({
      name: relative(repoRoot, path).split(sep).join("/"),
      language: LANGUAGE_BY_EXTENSION[extension],
      source: await readFile(path, "utf8"),
    });
  }

  if (cases.length === PROBES.length) {
    console.error("no .pw.* source was found; the corpus walk is broken");
    process.exit(1);
  }

  for (const { name, language, source } of cases) {
    const [left, right] = await Promise.all([
      outcome(committedFormatter, language, source, name),
      outcome(rebuiltFormatter, language, source, name),
    ]);
    if (JSON.stringify(left) !== JSON.stringify(right)) {
      differences.push({ name, detail: describe(left, right) });
    }
  }

  if (differences.length > 0) {
    console.error(
      [
        `the committed pwfmt.wasm does not behave like a fresh build (${differences.length} of ${cases.length} inputs differ).`,
        // Enough to name the change without scrolling a CI log past it.
        ...differences.slice(0, 5).map((d) => `  ${d.name}: ${d.detail}`),
        "Run npm run build:wasm and commit the result.",
      ].join("\n"),
    );
    process.exit(1);
  }

  console.log(
    `pwfmt.wasm behaves like a fresh build across ${cases.length} inputs`,
  );
} finally {
  await rm(scratch, { force: true, recursive: true });
}
