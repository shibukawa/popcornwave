// Tokenizes a source string with the extension's own grammars, using the same
// TextMate engine VS Code runs. External grammars the dialects fall back to
// (text.html.basic, source.sql, source.css, source.js) are deliberately absent:
// every scope rule:template-grammar-scopes fixes is matched before those
// includes, so a test that passes here passes with them loaded too.

import { readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);

// Both packages ship CommonJS; requiring them keeps their exports on the
// object rather than behind an interop default.
const oniguruma = require("vscode-oniguruma");
const textmate = require("vscode-textmate");

const here = dirname(fileURLToPath(import.meta.url));
const syntaxes = join(here, "..", "syntaxes");

const GRAMMARS = {
  "source.pw": "pw.tmLanguage.json",
  "source.pw.html": "pw-html.tmLanguage.json",
  "source.pw.sql": "pw-sql.tmLanguage.json",
  "source.pw.dynamo": "pw-dynamo.tmLanguage.json",
};

export const SCOPE_BY_EXTENSION = {
  ".pw.html": "source.pw.html",
  ".pw.sql": "source.pw.sql",
  ".pw.dynamo": "source.pw.dynamo",
};

let registryPromise;

async function createRegistry() {
  const wasm = await readFile(
    require.resolve("vscode-oniguruma/release/onig.wasm"),
  );
  await oniguruma.loadWASM(wasm.buffer);

  return new textmate.Registry({
    onigLib: Promise.resolve({
      createOnigScanner: (patterns) => new oniguruma.OnigScanner(patterns),
      createOnigString: (s) => new oniguruma.OnigString(s),
    }),
    loadGrammar: async (scopeName) => {
      const file = GRAMMARS[scopeName];
      if (!file) {
        return null;
      }
      const raw = await readFile(join(syntaxes, file), "utf8");
      return textmate.parseRawGrammar(raw, join(syntaxes, file));
    },
  });
}

function registry() {
  registryPromise ??= createRegistry();
  return registryPromise;
}

/**
 * @returns {Promise<Array<{line: number, text: string, scopes: string[]}>>}
 *   one entry per token, in document order, with empty-text tokens dropped.
 */
export async function tokenize(source, scopeName) {
  const grammar = await (await registry()).loadGrammar(scopeName);
  if (!grammar) {
    throw new Error(`grammar not found: ${scopeName}`);
  }

  const tokens = [];
  let state = textmate.INITIAL;
  const lines = source.split(/\r\n|\r|\n/);

  for (const [index, line] of lines.entries()) {
    const result = grammar.tokenizeLine(line, state);
    for (const token of result.tokens) {
      const text = line.slice(token.startIndex, token.endIndex);
      if (text.trim() === "") {
        continue;
      }
      tokens.push({ line: index + 1, text, scopes: token.scopes });
    }
    state = result.ruleStack;
  }

  return tokens;
}

/** The scopes covering the first occurrence of `text`. */
export function scopesOf(tokens, text) {
  const token = tokens.find((t) => t.text === text);
  if (!token) {
    const near = tokens
      .filter((t) => t.text.includes(text.trim()))
      .map((t) => JSON.stringify(t.text));
    throw new Error(
      `no token exactly matching ${JSON.stringify(text)}` +
        (near.length ? `; did you mean ${near.slice(0, 5).join(", ")}?` : ""),
    );
  }
  return token.scopes;
}

/** Every token whose text is `text`, in document order. */
export function allOf(tokens, text) {
  return tokens.filter((t) => t.text === text);
}

/** True when some scope on the token starts with `prefix`. */
export function hasScope(scopes, prefix) {
  return scopes.some((s) => s === prefix || s.startsWith(`${prefix}.`));
}
