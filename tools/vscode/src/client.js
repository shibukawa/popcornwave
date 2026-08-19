"use strict";

// What the language client is told to start and what it is told to watch.
//
// The construction of the client itself stays in extension.js, because it
// needs the VS Code API; everything decidable without it is here, so the shape
// of the server invocation is testable without an extension host.

const { DIALECT_BY_LANGUAGE } = require("./formatter");

/** The languages requirement:editor-language-registration registers. */
const LANGUAGES = Object.keys(DIALECT_BY_LANGUAGE);

/**
 * The command line api:cli-lsp is started with.
 *
 * --stdio is passed although stdio is the only transport, because a client
 * that names the transport it expects fails loudly against a future build that
 * has another one.
 *
 * @param {string} binary the resolved pw path
 * @param {{root?: string, log?: string}} options
 */
function serverInvocation(binary, { root = "", log = "" } = {}) {
  const args = ["lsp", "--stdio"];
  if (root !== "") {
    args.push(`--root=${root}`);
  }
  if (log !== "") {
    args.push(`--log=${log}`);
  }
  return { command: binary, args };
}

/**
 * The files the client watches on the server's behalf.
 *
 * A `popcornweb.toml` edit changes which directories carry which dialect, so
 * the server reloads its project model from it rather than being restarted.
 * The watch lives here because the client is what the editor gives a watcher
 * to; the server registers none of its own.
 */
const WATCHED_FILES = "**/popcornweb.toml";

/**
 * The document selector the client registers.
 *
 * It is scheme-bound to files: an untitled buffer has no project and no path,
 * and requirement:pw-language-server answers about a document it can name.
 */
function documentSelector() {
  return LANGUAGES.map((language) => ({ scheme: "file", language }));
}

module.exports = { LANGUAGES, WATCHED_FILES, serverInvocation, documentSelector };
