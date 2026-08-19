"use strict";

// Formatting through the project's own pw, per the delegated half of
// decision:formatter-delivery.
//
// The embedded module is a fixed tinybind version and the project pins its own
// through the pw it resolves, so the two can disagree about canonical form and
// the editor can canonicalize a file CI then rejects. Delegating removes that
// class: the bytes come from the same binary the project's own pw fmt runs.
//
// The embedded path stays for everything this one cannot reach — no project,
// no binary, an untrusted workspace — which is what
// requirement:editor-formatting promises and what stage 1 deliberately bought.

const { DIALECT_BY_LANGUAGE, FormatError } = require("./formatter");

/**
 * The probe that decides whether this path is usable.
 *
 * It is not a version check. requirement:editor-formatting dropped the
 * extension's own idempotence guard because templatefmt performs it from
 * v0.3.2, and what matters is whether the resolved pw has it, not what number
 * it prints. So the probe formats the source that was unstable before v0.3.2 —
 * a literal brace run in a style body, which gained a brace pair on every pass
 * — and requires a second pass to return the first one unchanged.
 */
const PROBE_SOURCE = [
  "export component ProbeStyleBraces(label: string): html {",
  "  <head>",
  "    <style>",
  ".demo { color: crimson }",
  "</style>",
  "  </head>",
  "  <p>{label}</p>",
  "}",
  "",
].join("\n");

/** The arguments that format one buffer of a dialect. */
function formatArgs(dialect) {
  // No file name: pw fmt --stdin formats one stream and refuses a path, so a
  // diagnostic names <stdin> and the caller supplies the document it asked
  // about.
  return ["fmt", `--stdin=${dialect}`];
}

/**
 * Reads a failed run into the finding the editor shows.
 *
 * The CLI writes "pw: fmt: <stdin>:3:1: message"; the prefix is its own and
 * the position belongs to the buffer the caller sent.
 */
function formatFailure(stderr, code) {
  const text = (stderr ?? "").trim();
  if (text === "") {
    return new FormatError(`pw fmt exited ${code} and said nothing`);
  }
  const message = text.replace(/^pw:\s*/, "").replace(/^fmt:\s*/, "");
  const position = /<stdin>:(\d+)(?::(\d+))?:\s*/.exec(message);
  if (!position) {
    return new FormatError(message);
  }
  return new FormatError(message.slice(position.index + position[0].length), {
    line: Number(position[1]),
  });
}

/**
 * Formats by running the resolved pw.
 *
 * @param {(args: string[], input: string) => Promise<{code: number, stdout: string, stderr: string}>} run
 */
class DelegatedFormatter {
  constructor(run) {
    this.run = run;
    this.checked = null;
  }

  /**
   * Decides once per session whether this path can be used, and says why not
   * when it cannot. The answer is cached: a probe on every format would double
   * the cost of the thing it is protecting.
   */
  async usable() {
    this.checked ??= await this.check();
    return this.checked;
  }

  async check() {
    let first;
    try {
      first = await this.run(formatArgs("html"), PROBE_SOURCE);
    } catch (error) {
      return { usable: false, reason: `pw could not be run: ${error.message}` };
    }
    if (first.code !== 0) {
      // An older pw has no fmt command, or no --stdin on it. Either way this
      // path does not exist for that binary.
      return {
        usable: false,
        reason: `pw fmt --stdin is unavailable (exit ${first.code}): ${(first.stderr ?? "").trim()}`,
      };
    }
    const second = await this.run(formatArgs("html"), first.stdout);
    if (second.code !== 0 || second.stdout !== first.stdout) {
      return {
        usable: false,
        reason:
          "the resolved pw formats this source differently on a second pass, " +
          "so it predates the idempotence guard the editor relies on",
      };
    }
    return { usable: true, reason: "" };
  }

  /** One formatting pass. Rejects with a FormatError, as the embedded one does. */
  async format(languageId, text) {
    const dialect = DIALECT_BY_LANGUAGE[languageId];
    if (!dialect) {
      throw new FormatError(`no dialect for language ${languageId}`);
    }
    const { code, stdout, stderr } = await this.run(formatArgs(dialect), text);
    if (code !== 0) {
      throw formatFailure(stderr, code);
    }
    return { text: stdout, changed: stdout !== text };
  }
}

module.exports = { DelegatedFormatter, PROBE_SOURCE, formatArgs, formatFailure };
