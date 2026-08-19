"use strict";

// The extension's only runtime code. Everything decidable without VS Code
// lives in formatter.js, binary.js, and client.js, so this file stays thin
// enough to read in one pass.

const { readFile } = require("node:fs/promises");
const { join } = require("node:path");

const vscode = require("vscode");
const { LanguageClient, TransportKind } = require("vscode-languageclient/node");

const { EmbeddedFormatter, FormatError } = require("./formatter");
const { resolvePw, INSTALL_HINT } = require("./binary");
const { CommandRunner } = require("./commands");
const { GENERATED_SCHEME, renderGenerated, routeItems, routePlaceholder, routeCaveat } = require("./views");
const { POLL_INTERVAL_MS, loopFinding, supersedes, loopStateURL } = require("./runtime");
const { LANGUAGES, WATCHED_FILES, serverInvocation, documentSelector } = require("./client");

/** @param {vscode.ExtensionContext} context */
function activate(context) {
  const output = vscode.window.createOutputChannel("Popcorn Web");
  context.subscriptions.push(output);

  registerFormatter(context, output);
  const server = new ServerSession(output);
  context.subscriptions.push(server);

  // The commands and the server resolve the same binary the same way, so an
  // editor that reports a diagnostic and an editor that runs pw are never
  // talking about two different installations.
  const runner = new CommandRunner(output, () => resolveWorkspace(output));
  runner.register(context);
  context.subscriptions.push(runner);

  // The surfaces the server answers with its own pw/* requests. Each one is
  // dead while the server is not running, and says so rather than failing.
  registerGeneratedView(context, server, output);
  registerRouteView(context, server);
  context.subscriptions.push(
    vscode.commands.registerCommand("popcornweb.previewStory", () => previewStory(server, output)),
  );
  const runtime = new RuntimeWatch(server, output);
  runtime.start();
  context.subscriptions.push(runtime);

  announceHost(output);

  context.subscriptions.push(
    vscode.commands.registerCommand("popcornweb.showOutput", () => output.show()),
    vscode.commands.registerCommand("popcornweb.restartLanguageServer", () => server.restart()),
    // A workspace becoming trusted is the moment a process is allowed at all,
    // per policy:editor-tool-execution, so the server starts there rather than
    // asking the developer to reload the window.
    vscode.workspace.onDidGrantWorkspaceTrust(() => server.start()),
    vscode.workspace.onDidChangeConfiguration((event) => {
      if (event.affectsConfiguration("popcornweb")) {
        server.restart();
        runtime.start();
      }
    }),
  );

  server.start();
}

/**
 * The folder to run in and the binary to run, or null with the reason already
 * in the output channel.
 *
 * A command needs a folder as much as it needs a binary: pw finds the project
 * by walking up from where it runs, so a command started from nowhere would
 * find nothing.
 */
function resolveWorkspace(output) {
  const folders = (vscode.workspace.workspaceFolders ?? [])
    .filter((folder) => folder.uri.scheme === "file")
    .map((folder) => folder.uri.fsPath);
  if (folders.length === 0) {
    output.appendLine("No folder is open, so there is no project for pw to find.");
    return null;
  }
  const settings = vscode.workspace.getConfiguration("popcornweb");
  const resolved = resolvePw({
    folders,
    env: process.env,
    configured: settings.get("pw.path", "").trim(),
  });
  if (!resolved.path) {
    output.appendLine(resolved.reason);
    return null;
  }
  return { binary: resolved.path, folder: folders[0] };
}

/**
 * The read-only view of requirement:editor-generated-peek.
 *
 * A virtual document rather than the file itself: policy:generated-artifacts
 * owns the file, and opening it would offer an edit the next generation throws
 * away.
 */
function registerGeneratedView(context, server, output) {
  const documents = new Map();
  const changed = new vscode.EventEmitter();

  context.subscriptions.push(
    vscode.workspace.registerTextDocumentContentProvider(GENERATED_SCHEME, {
      onDidChange: changed.event,
      provideTextDocumentContent: (uri) => documents.get(uri.toString()) ?? "",
    }),
    changed,
    vscode.commands.registerCommand("popcornweb.peekGenerated", async () => {
      const editor = vscode.window.activeTextEditor;
      if (!editor) {
        return;
      }
      const fragment = await server.request("pw/generatedFor", {
        textDocument: { uri: editor.document.uri.toString() },
        position: editor.selection.active,
      });
      if (!fragment) {
        output.appendLine("The language server is not running, so there is nothing to show.");
        output.show();
        return;
      }
      // One stable uri per declaration, so asking again refreshes the view
      // rather than opening another tab of the same thing.
      const uri = vscode.Uri.parse(
        `${GENERATED_SCHEME}:${fragment.declaration || "position"}.go`,
      );
      documents.set(uri.toString(), renderGenerated(fragment));
      changed.fire(uri);
      const document = await vscode.workspace.openTextDocument(uri);
      await vscode.window.showTextDocument(document, { preview: true, viewColumn: vscode.ViewColumn.Beside });
      await vscode.languages.setTextDocumentLanguage(document, "go");
    }),
  );
}

/** The tree of requirement:editor-route-explorer. */
function registerRouteView(context, server) {
  const changed = new vscode.EventEmitter();
  let report = null;

  const provider = {
    onDidChangeTreeData: changed.event,
    getChildren: async () => {
      report = await server.request("pw/routes", {});
      const placeholder = routePlaceholder(report);
      if (placeholder) {
        return [{ placeholder }];
      }
      const items = routeItems(report);
      const caveat = routeCaveat(report);
      // The caveat is a row rather than a tooltip: a view that silently
      // covers half the URL space reads as if it covered all of it.
      return caveat ? [...items, { placeholder: `Not shown: ${caveat}` }] : items;
    },
    getTreeItem: (entry) => {
      if (entry.placeholder) {
        const item = new vscode.TreeItem(entry.placeholder);
        item.tooltip = entry.placeholder;
        return item;
      }
      const item = new vscode.TreeItem(entry.path);
      item.description = entry.description;
      item.tooltip = entry.tooltip;
      item.resourceUri = vscode.Uri.parse(entry.pageUri);
      item.command = {
        command: "vscode.open",
        title: "Open the page template",
        arguments: [vscode.Uri.parse(entry.pageUri)],
      };
      return item;
    },
  };

  context.subscriptions.push(
    vscode.window.registerTreeDataProvider("popcornweb.routes", provider),
    vscode.commands.registerCommand("popcornweb.refreshRoutes", () => changed.fire()),
    changed,
  );
}

/**
 * requirement:editor-story-preview: the extension computes which story URL the
 * caret corresponds to and opens it. It renders nothing itself, which is the
 * rule requirement:editor-tasks already states for the dev console panes.
 */
async function previewStory(server, output) {
  const editor = vscode.window.activeTextEditor;
  if (!editor) {
    return;
  }
  const target = await server.request("pw/storyFor", {
    textDocument: { uri: editor.document.uri.toString() },
    position: editor.selection.active,
  });
  if (!target) {
    output.appendLine("The language server is not running, so the story URL is not known.");
    output.show();
    return;
  }
  if (target.status !== "ok") {
    output.appendLine(`No story preview: ${target.message}`);
    output.show();
    return;
  }
  // The dev loop is never started from here, per policy:editor-tool-execution.
  // If it is not running the browser says so, which is a clearer answer than
  // this extension guessing at a port it did not open.
  output.appendLine(`Opening ${target.url}. It needs pw dev to be running.`);
  await vscode.env.openExternal(vscode.Uri.parse(target.url));
}

/**
 * requirement:editor-web-host's statement of absence.
 *
 * A browser-hosted editor runs no process at all, so everything past stage 1
 * is unavailable there — and an extension that simply does less without saying
 * so reads as a broken language server, which is what
 * requirement:extension-distribution asks the readme to prevent and what this
 * makes a runtime statement instead.
 */
function announceHost(output) {
  const remote = vscode.env.uiKind === vscode.UIKind?.Web;
  if (!remote) {
    return;
  }
  output.appendLine(
    "Running in a browser-hosted editor. Highlighting and formatting work here, " +
      "because the formatter is a WebAssembly module inside the extension. " +
      "Diagnostics, the outline, navigation, and the pw commands need a pw binary, " +
      "and a web host runs no process, so none of them are available.",
  );
}

/**
 * requirement:editor-runtime-diagnostics, for the failures the dev loop
 * reports.
 *
 * It is off unless the developer switches it on, and while it is off nothing
 * is contacted at all. That is the one exception to the network clause of
 * policy:editor-tool-execution, and its bounds are the reason it is one: a
 * loopback request to a console the developer started, reading a record that
 * never leaves the machine.
 */
class RuntimeWatch {
  constructor(server, output) {
    this.server = server;
    this.output = output;
    this.diagnostics = vscode.languages.createDiagnosticCollection("popcornweb-runtime");
    this.timer = null;
    this.previous = null;
    this.announced = false;
  }

  start() {
    this.stop();
    const settings = vscode.workspace.getConfiguration("popcornweb");
    if (!settings.get("runtimeDiagnostics.enabled", false)) {
      return;
    }
    this.timer = setInterval(() => void this.poll(), POLL_INTERVAL_MS);
  }

  async poll() {
    const info = await this.server.request("pw/project", {});
    const url = loopStateURL(info?.consoleUrl);
    if (!url) {
      return;
    }
    let state;
    try {
      const response = await fetch(url, { signal: AbortSignal.timeout(1000) });
      if (!response.ok) {
        return;
      }
      state = await response.json();
    } catch {
      // The loop is not running, which is the ordinary state. Saying so on
      // every poll would fill the channel with the absence of news.
      this.clear();
      return;
    }
    this.apply(state, info.root);
  }

  apply(state, root) {
    const finding = loopFinding(state);
    if (finding.kind === "clear") {
      this.clear();
      return;
    }
    if (!supersedes(this.previous, finding)) {
      return;
    }
    this.previous = finding;
    this.diagnostics.clear();

    // A failure with no file is still a failure. Reporting it in the channel
    // rather than dropping it is what keeps the view honest.
    if (!finding.file) {
      this.output.appendLine(`Dev loop: ${finding.message}`);
      return;
    }
    const path = finding.file.startsWith("/") ? finding.file : `${root}/${finding.file}`;
    const line = Math.max(0, finding.line - 1);
    const column = Math.max(0, finding.column - 1);
    const diagnostic = new vscode.Diagnostic(
      new vscode.Range(line, column, line, Number.MAX_SAFE_INTEGER),
      finding.message,
      0,
    );
    // A distinct source, so a runtime finding is never mistaken for one
    // api:cli-generate would have produced.
    diagnostic.source = "pw dev";
    this.diagnostics.set(vscode.Uri.file(path), [diagnostic]);

    if (!this.announced) {
      this.announced = true;
      this.output.appendLine(
        "Reporting dev loop failures. This reads the console over loopback and sends nothing anywhere.",
      );
    }
  }

  clear() {
    if (this.previous) {
      this.previous = null;
      this.diagnostics.clear();
    }
  }

  stop() {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
    this.clear();
  }

  dispose() {
    this.stop();
    this.diagnostics.dispose();
  }
}

function registerFormatter(context, output) {
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
}

/**
 * The api:cli-lsp client and the conditions under which it may run.
 *
 * Every reason not to start is reported once and leaves the extension in its
 * stage-1 behavior, which is the promise requirement:editor-web-host and
 * policy:editor-tool-execution both make: a missing binary costs the language
 * features and nothing else.
 */
class ServerSession {
  constructor(output) {
    this.output = output;
    this.client = null;
    this.reported = "";
  }

  async start() {
    if (this.client) {
      return;
    }
    const settings = vscode.workspace.getConfiguration("popcornweb");
    if (!settings.get("languageServer.enabled", true)) {
      this.reportOnce("The language server is disabled by popcornweb.languageServer.enabled.");
      return;
    }
    if (!vscode.workspace.isTrusted) {
      this.reportOnce(
        "This workspace is not trusted, so no process is started. " +
          "Highlighting and formatting work; diagnostics and the outline do not.",
      );
      return;
    }

    const folders = (vscode.workspace.workspaceFolders ?? [])
      .filter((folder) => folder.uri.scheme === "file")
      .map((folder) => folder.uri.fsPath);
    const resolved = resolvePw({
      folders,
      env: process.env,
      configured: settings.get("pw.path", "").trim(),
    });
    if (!resolved.path) {
      this.reportOnce(resolved.reason);
      return;
    }

    const invocation = serverInvocation(resolved.path, {
      root: folders[0] ?? "",
      log: settings.get("languageServer.log", "").trim(),
    });
    this.output.appendLine(
      `Starting ${invocation.command} ${invocation.args.join(" ")} (found through ${resolved.source}).`,
    );

    this.client = new LanguageClient(
      "popcornweb",
      "Popcorn Web Language Server",
      {
        run: { ...invocation, transport: TransportKind.stdio },
        debug: { ...invocation, transport: TransportKind.stdio },
      },
      {
        documentSelector: documentSelector(),
        outputChannel: this.output,
        synchronize: {
          fileEvents: vscode.workspace.createFileSystemWatcher(WATCHED_FILES),
        },
        // The server holds only what the client sends it, so a failure is
        // recoverable by restarting rather than by reloading the window.
        initializationFailedHandler: (error) => {
          this.reportOnce(`The language server did not start: ${error.message}`);
          return false;
        },
      },
    );

    try {
      await this.client.start();
      this.reported = "";
    } catch (error) {
      this.client = null;
      this.reportOnce(
        `The language server did not start: ${error.message}. ` +
          `Check that ${resolved.path} is a pw new enough to have the lsp command, or install one with \`${INSTALL_HINT}\`.`,
      );
    }
  }

  /**
   * Sends one of this server's own pw/* requests, or reports null when the
   * server is not running. Null is a state a caller has to render, so it is
   * returned rather than thrown.
   */
  async request(method, params) {
    if (!this.client) {
      return null;
    }
    try {
      return await this.client.sendRequest(method, params);
    } catch (error) {
      this.output.appendLine(`${method} failed: ${error.message}`);
      return null;
    }
  }

  async stop() {
    const client = this.client;
    this.client = null;
    if (client) {
      await client.stop().catch(() => {});
    }
  }

  async restart() {
    await this.stop();
    this.reported = "";
    await this.start();
  }

  /**
   * Reports a reason once. A reason repeated on every open would be noise for
   * a developer who has already decided not to install pw.
   */
  reportOnce(reason) {
    if (this.reported === reason) {
      return;
    }
    this.reported = reason;
    this.output.appendLine(reason);
  }

  dispose() {
    void this.stop();
  }
}

function report(error, document, output) {
  const where = error instanceof FormatError && error.line ? ` (line ${error.line})` : "";
  const message = `Popcorn Web: the file was not formatted${where}. ${error.message}`;
  output.appendLine(`${document.fileName}: ${error.message}`);
  vscode.window.showWarningMessage(message, "Show details").then((choice) => {
    if (choice === "Show details") {
      output.show();
    }
  });
}

function deactivate() {}

module.exports = { activate, deactivate };
