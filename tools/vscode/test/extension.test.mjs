// Exercises src/extension.js against a stub VS Code API.
//
// It cannot prove the extension works in a real host, but it does prove the
// glue: that a provider is registered per language, that an unchanged buffer
// produces no edit, that a changed one produces exactly one whole-document
// replacement, and that every failure path returns an empty edit list so the
// buffer is left alone.

import assert from "node:assert/strict";
import Module from "node:module";
import { createRequire } from "node:module";
import { test } from "node:test";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const extensionRoot = join(here, "..");

/** Minimal stand-ins for the VS Code API surface extension.js touches. */
function createStubVscode() {
  const state = { output: [], warnings: [], providers: [], commands: [] };

  class Position {
    constructor(offset) {
      this.offset = offset;
    }
  }
  class Range {
    constructor(start, end) {
      this.start = start;
      this.end = end;
    }
  }

  const vscode = {
    Range,
    Position,
    TextEdit: {
      replace: (range, newText) => ({ kind: "replace", range, newText }),
    },
    window: {
      createOutputChannel: () => ({
        appendLine: (line) => state.output.push(line),
        show: () => {},
        dispose: () => {},
      }),
      showWarningMessage: (message) => {
        state.warnings.push(message);
        return Promise.resolve(undefined);
      },
    },
    languages: {
      registerDocumentFormattingEditProvider: (language, provider) => {
        state.providers.push({ language, provider });
        return { dispose() {} };
      },
    },
    commands: {
      registerCommand: (command) => {
        state.commands.push(command);
        return { dispose() {} };
      },
    },
  };

  return { vscode, state };
}

function loadExtension(vscode) {
  const require = createRequire(import.meta.url);
  const original = Module._load;
  Module._load = function (request, parent, isMain) {
    if (request === "vscode") {
      return vscode;
    }
    return original.call(this, request, parent, isMain);
  };
  try {
    const path = join(extensionRoot, "src", "extension.js");
    delete require.cache[require.resolve(path)];
    return require(path);
  } finally {
    Module._load = original;
  }
}

function makeDocument(languageId, text, fileName = "/tmp/example.pw.sql") {
  return {
    languageId,
    fileName,
    getText: () => text,
    positionAt: (offset) => ({ offset }),
  };
}

function activateStub() {
  const { vscode, state } = createStubVscode();
  const extension = loadExtension(vscode);
  const context = { extensionPath: extensionRoot, subscriptions: [] };
  extension.activate(context);
  return { state, context };
}

test("a formatting provider is registered for each dialect", () => {
  const { state } = activateStub();
  assert.deepEqual(
    state.providers.map((p) => p.language).sort(),
    ["pw-dynamo", "pw-html", "pw-sql"],
  );
  assert.deepEqual(state.commands, ["popcornwave.showOutput"]);
});

test("an unformatted buffer yields one whole-document replacement", async () => {
  const { state } = activateStub();
  const { provider } = state.providers.find((p) => p.language === "pw-sql");

  const text = "package q\nexport statement F():sql.exec{DELETE FROM t}\n";
  const edits = await provider.provideDocumentFormattingEdits(makeDocument("pw-sql", text));

  assert.equal(edits.length, 1);
  assert.equal(edits[0].kind, "replace");
  assert.equal(edits[0].range.start.offset, 0);
  assert.equal(edits[0].range.end.offset, text.length);
  assert.match(edits[0].newText, /export statement F\(\): sql\.exec \{/);
  assert.equal(state.warnings.length, 0);
});

test("a canonical buffer yields no edit at all", async () => {
  const { state } = activateStub();
  const { provider } = state.providers.find((p) => p.language === "pw-sql");

  const canonical = "package q\n\nexport statement F(): sql.exec {\n  DELETE FROM t\n}\n";
  const edits = await provider.provideDocumentFormattingEdits(
    makeDocument("pw-sql", canonical),
  );

  assert.deepEqual(edits, [], "an already canonical buffer must not touch the undo stack");
});

test("the delivery path is announced once per session", async () => {
  const { state } = activateStub();
  const { provider } = state.providers.find((p) => p.language === "pw-sql");
  const document = makeDocument("pw-sql", "package q\nexport statement F():sql.exec{DELETE FROM t}\n");

  await provider.provideDocumentFormattingEdits(document);
  await provider.provideDocumentFormattingEdits(document);

  const announcements = state.output.filter((line) => line.includes("embedded tinybind formatter"));
  assert.equal(announcements.length, 1);
});

test("a syntax error leaves the buffer alone and warns once", async () => {
  const { state } = activateStub();
  const { provider } = state.providers.find((p) => p.language === "pw-html");

  const edits = await provider.provideDocumentFormattingEdits(
    makeDocument("pw-html", "export component X(): html {\n<p>unclosed\n", "/tmp/x.pw.html"),
  );

  assert.deepEqual(edits, []);
  assert.equal(state.warnings.length, 1);
  assert.match(state.warnings[0], /was not formatted \(line 3\)/);
});

test("a style body that used to be refused now passes through untouched", async () => {
  // Through tinybind v0.3.1 this source grew a brace pair per pass and the
  // extension refused it. It is now canonical, so the provider returns no edit
  // and warns about nothing.
  const { state } = activateStub();
  const { provider } = state.providers.find((p) => p.language === "pw-html");

  const source = [
    "export component B(label: string): html {",
    "  <head>",
    "    <style>",
    ".demo { color: crimson }",
    "</style>",
    "  </head>",
    "  <p>{label}</p>",
    "}",
    "",
  ].join("\n");

  const edits = await provider.provideDocumentFormattingEdits(
    makeDocument("pw-html", source, "/tmp/b.pw.html"),
  );

  assert.deepEqual(edits, [], "the source is already canonical, so there is nothing to edit");
  assert.deepEqual(state.warnings, []);
});
