"use strict";

// Runs the embedded formatter, with the guard requirement:editor-formatting
// requires around every result.
//
// The module is compiled once per session and instantiated once per format.
// Instantiation is cheap and gives each run a clean Go heap and a clean exit
// path, which is what makes proc_exit safe to implement as a throw.

const { createWasi } = require("./wasi");

const DIALECT_BY_LANGUAGE = {
  "pw-html": "html",
  "pw-sql": "sql",
  "pw-dynamo": "dynamo",
};

class FormatError extends Error {
  constructor(message, { line = null } = {}) {
    super(message);
    this.name = "FormatError";
    this.line = line;
  }
}

/** Parses "<name>:12:3: message" or ":12: message" out of a diagnostic. */
function lineOf(diagnostic) {
  const match = /:(\d+)(?::\d+)?:/.exec(diagnostic);
  return match ? Number(match[1]) : null;
}

class EmbeddedFormatter {
  /** @param {() => Promise<Uint8Array>} readModule reads the .wasm bytes */
  constructor(readModule) {
    this._readModule = readModule;
    this._compiled = null;
  }

  async _module() {
    this._compiled ??= WebAssembly.compile(await this._readModule());
    return this._compiled;
  }

  /**
   * One formatting pass. Resolves with the formatted text, or rejects with a
   * FormatError carrying the module's own diagnostic.
   */
  async formatOnce(dialect, text, fileName = "") {
    const module = await this._module();

    let memory;
    const wasi = createWasi({
      args: ["pwfmt", dialect, fileName],
      stdin: new TextEncoder().encode(text),
      memory: () => memory,
    });

    const instance = await WebAssembly.instantiate(module, wasi.imports);
    memory = instance.exports.memory;

    try {
      instance.exports._start();
    } catch (error) {
      // proc_exit is the module's normal way to finish a failed run, so an
      // Exit unwind is expected and the exit code below decides the outcome.
      if (!(error instanceof wasi.Exit)) {
        throw new FormatError(`the formatter crashed: ${error.message}`);
      }
    }

    const { exitCode, stdout, stderr } = wasi.result();
    const decoder = new TextDecoder();
    if (exitCode !== 0) {
      const diagnostic = decoder.decode(stderr).trim() || "the formatter failed";
      throw new FormatError(diagnostic, { line: lineOf(diagnostic) });
    }
    return decoder.decode(stdout);
  }

  /**
   * Formats one buffer.
   *
   * The idempotence guard requirement:editor-formatting asks for lives inside
   * templatefmt from tinybind v0.3.2: the library formats twice itself and
   * returns an error rather than a result that differs between the passes. It
   * sits closer to the AST than this file can, so duplicating it here would
   * cost a second round trip to defend against that guard being broken. The
   * version floor is what makes relying on it safe, and wasm/go.mod is where
   * the floor is recorded.
   *
   * @returns {Promise<{text: string, changed: boolean}>}
   */
  async format(languageId, text, fileName = "") {
    const dialect = DIALECT_BY_LANGUAGE[languageId];
    if (!dialect) {
      throw new FormatError(`no dialect for language ${languageId}`);
    }

    const formatted = await this.formatOnce(dialect, text, fileName);
    return { text: formatted, changed: formatted !== text };
  }
}

module.exports = {
  EmbeddedFormatter,
  FormatError,
  DIALECT_BY_LANGUAGE,
};
