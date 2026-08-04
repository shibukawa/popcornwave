"use strict";

// The extension's only runtime code. Everything decidable without VS Code
// lives in formatter.js, so this file stays thin enough to read in one pass.

const { readFile } = require("node:fs/promises");
const { join } = require("node:path");

const vscode = require("vscode");

const { EmbeddedFormatter, FormatError, DIALECT_BY_LANGUAGE } = require("./formatter");

const LANGUAGES = Object.keys(DIALECT_BY_LANGUAGE);

/** @param {vscode.ExtensionContext} context */
function activate(context) {
  const output = vscode.window.createOutputChannel("Popcorn Wave");
  context.subscriptions.push(output);

  const formatter = new EmbeddedFormatter(() =>
    readFile(join(context.extensionPath, "wasm", "pwfmt.wasm")),
  );

  // decision:formatter-delivery names two paths and this version ships one.
  // Saying which produced a result makes a surprising diff traceable, and
  // leaves an obvious place for the delegated path to announce itself.
  let announced = false;
  const announce = () => {
    if (announced) {
      return;
    }
    announced = true;
    output.appendLine(
      "Formatting with the embedded tinybind formatter. " +
        "The pw fmt delegation is not available in this version, so a project " +
        "pinning a different tinybind may format differently in CI.",
    );
  };

  const provider = {
    async provideDocumentFormattingEdits(document) {
      try {
        const { text, changed } = await formatter.format(
          document.languageId,
          document.getText(),
          document.fileName,
        );
        announce();
        if (!changed) {
          // No edit at all, so an already canonical buffer never touches the
          // undo stack and never marks the document dirty.
          return [];
        }
        const whole = new vscode.Range(
          document.positionAt(0),
          document.positionAt(document.getText().length),
        );
        return [vscode.TextEdit.replace(whole, text)];
      } catch (error) {
        report(error, document, output);
        // An empty edit list leaves the buffer exactly as it was, which is the
        // requirement:editor-formatting promise on every failure path.
        return [];
      }
    },
  };

  for (const language of LANGUAGES) {
    context.subscriptions.push(
      vscode.languages.registerDocumentFormattingEditProvider(language, provider),
    );
  }

  context.subscriptions.push(
    vscode.commands.registerCommand("popcornwave.showOutput", () => output.show()),
  );
}

function report(error, document, output) {
  const where = error instanceof FormatError && error.line ? ` (line ${error.line})` : "";
  const message = `Popcorn Wave: the file was not formatted${where}. ${error.message}`;
  output.appendLine(`${document.fileName}: ${error.message}`);
  vscode.window.showWarningMessage(message, "Show details").then((choice) => {
    if (choice === "Show details") {
      output.show();
    }
  });
}

function deactivate() {}

module.exports = { activate, deactivate };
