"use strict";

// Finding the pw binary the language server runs from.
//
// policy:editor-tool-execution fixes the order and the prohibitions: the
// workspace Devbox environment, then PATH, then a user-configured absolute
// path, and never a download, an install, or an update. Nothing here starts a
// process; resolution is a file lookup, because deciding which binary to run
// by running one is the thing an editor must not do on open.

const { isAbsolute, join } = require("node:path");
const { accessSync, constants, statSync } = require("node:fs");

// Where devbox puts the packages a project declares. It is a path rather than
// a `devbox run` invocation for the reason above: reading a directory is not
// running the workspace's own tooling.
const DEVBOX_PROFILE = join(".devbox", "nix", "profile", "default", "bin");

/** The install instruction requirement:cli-distribution publishes. */
const INSTALL_HINT = "go install github.com/shibukawa/popcornweb/cmd/pw@latest";

/** Returns true when path exists and can be executed. */
function isExecutableFile(path) {
  try {
    if (!statSync(path).isFile()) {
      return false;
    }
    accessSync(path, constants.X_OK);
    return true;
  } catch {
    return false;
  }
}

/**
 * Resolves the pw binary.
 *
 * @param {object} options
 * @param {string[]} options.folders workspace folder paths, in the editor's order
 * @param {Record<string, string | undefined>} options.env the extension host environment
 * @param {string} options.configured the user's absolute path setting, or ""
 * @param {string} options.platform process.platform, taken as an argument so the
 *   Windows name is testable from anywhere
 * @param {(path: string) => boolean} [options.isExecutable] injected for tests
 * @returns {{path: string, source: string} | {path: null, reason: string}}
 */
function resolvePw({
  folders = [],
  env = {},
  configured = "",
  platform = process.platform,
  isExecutable = isExecutableFile,
} = {}) {
  const name = platform === "win32" ? "pw.exe" : "pw";

  for (const folder of folders) {
    const candidate = join(folder, DEVBOX_PROFILE, name);
    if (isExecutable(candidate)) {
      return { path: candidate, source: "the workspace devbox environment" };
    }
  }

  // The separator follows the platform argument rather than this process's,
  // because a Windows entry holds a drive letter and splitting it on a colon
  // turns one directory into two that do not exist.
  const search = env.PATH ?? env.Path ?? "";
  for (const entry of search.split(platform === "win32" ? ";" : ":")) {
    if (entry === "") {
      continue;
    }
    const candidate = join(entry, name);
    if (isExecutable(candidate)) {
      return { path: candidate, source: "PATH" };
    }
  }

  if (configured !== "") {
    // A relative path here would resolve against whatever directory the
    // extension host happens to have, which is not a location the developer
    // can reason about; policy:editor-tool-execution asks for an absolute one.
    if (!isAbsolute(configured)) {
      return { path: null, reason: `popcornweb.pw.path must be absolute, and ${configured} is not.` };
    }
    if (isExecutable(configured)) {
      return { path: configured, source: "popcornweb.pw.path" };
    }
    return { path: null, reason: `popcornweb.pw.path points at ${configured}, which is not an executable file.` };
  }

  return {
    path: null,
    reason:
      "pw was not found in the workspace devbox environment or on PATH. " +
      `Install it with \`${INSTALL_HINT}\`, or set popcornweb.pw.path to its absolute location. ` +
      "Highlighting and formatting keep working without it.",
  };
}

module.exports = { resolvePw, isExecutableFile, DEVBOX_PROFILE, INSTALL_HINT };
