"use strict";

// requirement:editor-runtime-diagnostics: the failures that only exist while
// the application is being built and run.
//
// The source is the requirement:dev-console loop state, which carries the
// failure the loop is currently reporting together with the file, line, and
// column it named. Nothing is inferred: a failure with no position is reported
// against the project rather than placed somewhere plausible.
//
// It is off by default and it contacts nothing until it is switched on. See
// the network clause of policy:editor-tool-execution, which this is the one
// stated exception to: a loopback request to a console the developer started,
// carrying nothing off the machine.

/** How often the loop state is read while the feature is on. */
const POLL_INTERVAL_MS = 2000;

/**
 * Converts one loop-state record into what a client should show.
 *
 * @returns {{kind: "clear"} | {kind: "finding", file: string|null, line: number,
 *   column: number, message: string, build: string}}
 */
function loopFinding(state) {
  if (!state || !state.diagnostic || !state.diagnostic.text) {
    // A loop that is not failing has nothing to report, and the previous
    // finding described code that may no longer exist.
    return { kind: "clear" };
  }
  const { text, file, line, column } = state.diagnostic;
  return {
    kind: "finding",
    // Zero means the diagnostic named no location, not line zero.
    file: file || null,
    line: line > 0 ? line : 1,
    column: column > 0 ? column : 1,
    message: `${state.phase || "the dev loop"}: ${text}`,
    build: state.build ?? "",
  };
}

/**
 * Whether a finding replaces what is on screen.
 *
 * A finding from a previous build describes code that may no longer exist, so
 * a build change clears before it publishes.
 */
function supersedes(previous, finding) {
  if (!previous) {
    return true;
  }
  return previous.build !== finding.build || previous.message !== finding.message;
}

/** The URL the loop state is read from. */
function loopStateURL(consoleUrl) {
  if (!consoleUrl) {
    return null;
  }
  return `${consoleUrl.replace(/\/+$/, "")}/api/loop-state`;
}

module.exports = { POLL_INTERVAL_MS, loopFinding, supersedes, loopStateURL };
