// Rebuilds the formatter module and fails when the result differs from the
// committed artifact, so the .wasm in the tree cannot drift from wasm/main.go
// or from the tinybind version its go.mod pins.
//
// Run by CI rather than by npm test, because it needs a TinyGo toolchain that
// a contributor editing only the grammars should not have to install.

import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { copyFile, readFile, rename, rm } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const wasmDir = join(here, "..", "wasm");
const committed = join(wasmDir, "pwfmt.wasm");
const backup = join(wasmDir, "pwfmt.wasm.committed");

const digest = async (path) =>
  createHash("sha256")
    .update(await readFile(path))
    .digest("hex");

const before = await digest(committed);
await copyFile(committed, backup);

try {
  execFileSync("sh", [join(wasmDir, "build.sh")], { stdio: "inherit" });
  const after = await digest(committed);
  if (before !== after) {
    // Put the committed bytes back, so a failing check leaves no diff behind.
    await rename(backup, committed);
    console.error(
      [
        "the committed pwfmt.wasm does not match a fresh build.",
        `  committed: ${before}`,
        `  rebuilt:   ${after}`,
        "Run npm run build:wasm and commit the result.",
        "A TinyGo version difference also produces this; wasm/TOOLCHAIN records the one used.",
      ].join("\n"),
    );
    process.exit(1);
  }
  console.log(`pwfmt.wasm matches a fresh build (${before.slice(0, 12)})`);
} finally {
  await rm(backup, { force: true });
}
