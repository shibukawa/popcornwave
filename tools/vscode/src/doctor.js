"use strict";

// Turning a `pw doctor --format=json` report into editor diagnostics.
//
// requirement:editor-tasks asks for the findings on the files and configuration
// lines the report names. The report names them in prose — evidence is a path,
// a `<key> in <file>` pair, or something that is neither — so the mapping here
// is a reading of that text, and a finding whose evidence names no file lands
// on the project configuration rather than nowhere.

const { existsSync, readFileSync } = require("node:fs");
const { isAbsolute, join } = require("node:path");

/** The LSP severities, which are what a VS Code diagnostic uses too. */
const SEVERITY = { error: 1, warning: 2, note: 3 };

/**
 * Locates a finding.
 *
 * @param {string} root the workspace folder the report was produced in
 * @param {string} evidence the finding's evidence string
 * @param {string} configPath the environment's configuration file, if it has one
 * @param {(path: string) => string | null} read reads a file, or null
 * @returns {{file: string, line: number}} a one-based line
 */
function locate(root, evidence, configPath, read) {
  const fallback = () => ({ file: configPath || "popcornweb.toml", line: 1 });
  if (!evidence) {
    return fallback();
  }

  // "<path>:<line>", which is how a finding about a source names its place.
  const positioned = /^(.+):(\d+)$/.exec(evidence);
  if (positioned && exists(root, positioned[1])) {
    return { file: positioned[1], line: Number(positioned[2]) };
  }

  // A bare path, which is how a finding about a whole file names it.
  if (exists(root, evidence)) {
    return { file: evidence, line: 1 };
  }

  // "<key> in <file>", which is how a configuration finding names its value.
  const keyed = /^(\S+) in (.+)$/.exec(evidence);
  if (keyed && exists(root, keyed[2])) {
    return { file: keyed[2], line: lineOfKey(read(join(root, keyed[2])), keyed[1]) };
  }

  // A dotted key with no file: the project configuration is where it lives.
  if (/^[a-z0-9_]+(\.[a-z0-9_]+)+$/i.test(evidence)) {
    const file = configPath || "popcornweb.toml";
    return { file, line: lineOfKey(read(join(root, file)), evidence) };
  }

  return fallback();
}

function exists(root, candidate) {
  if (candidate.includes("\n") || candidate === "") {
    return false;
  }
  return existsSync(isAbsolute(candidate) ? candidate : join(root, candidate));
}

/**
 * The one-based line a TOML key is written on, or 1 when it is not found.
 *
 * The key may be dotted, and the file may spell it as a section header with a
 * bare key under it, so both forms are looked for. This is a text scan rather
 * than a parse: the position of a key is a property of the file's layout, and a
 * parsed document has already thrown that away.
 */
function lineOfKey(source, key) {
  if (!source) {
    return 1;
  }
  const parts = key.split(".");
  const leaf = parts[parts.length - 1];
  const section = parts.slice(0, -1).join(".");

  const lines = source.split("\n");
  let current = "";
  let sectionLine = 0;
  for (let index = 0; index < lines.length; index += 1) {
    const text = lines[index].trim();
    const header = /^\[+([^\]]+)\]+$/.exec(text);
    if (header) {
      current = header[1].trim();
      if (current === section || current === key) {
        sectionLine = index + 1;
      }
      continue;
    }
    const assignment = /^([A-Za-z0-9_."-]+)\s*=/.exec(text);
    if (!assignment) {
      continue;
    }
    const name = assignment[1].replace(/"/g, "");
    if (name === key || (current === section && name === leaf)) {
      return index + 1;
    }
  }
  // The section exists and the key is defaulted rather than written. Its header
  // is the closest true position, and pointing at line 1 instead would send the
  // reader to the top of a file they then have to search.
  return sectionLine || 1;
}

/**
 * Converts a report into diagnostics grouped by file.
 *
 * @returns {Map<string, {line: number, severity: number, message: string, code: string, docs: string}[]>}
 */
function reportDiagnostics(root, report, read = readIfPresent) {
  const byFile = new Map();
  for (const environment of report?.environments ?? []) {
    for (const finding of environment.findings ?? []) {
      const { file, line } = locate(root, finding.evidence, environment.config_path, read);
      const entries = byFile.get(file) ?? [];
      entries.push({
        line,
        severity: SEVERITY[finding.severity] ?? SEVERITY.note,
        // The environment is part of the finding: the same key is fine in dev
        // and wrong in prod, and a diagnostic that does not say which one it is
        // about sends the reader to change the wrong file.
        message: `[${environment.env}] ${finding.message}`,
        code: finding.id,
        docs: finding.docs ?? "",
      });
      byFile.set(file, entries);
    }
  }
  return byFile;
}

/**
 * What the report could not determine, which is the half a clean-looking report
 * would otherwise hide.
 */
function reportLimits(report) {
  return (report?.limits ?? []).map(
    (limit) => `${limit.Subject ?? limit.subject}: ${limit.Reason ?? limit.reason} — ${limit.Effect ?? limit.effect}`,
  );
}

function readIfPresent(path) {
  try {
    return readFileSync(path, "utf8");
  } catch {
    return null;
  }
}

module.exports = { SEVERITY, locate, lineOfKey, reportDiagnostics, reportLimits };
