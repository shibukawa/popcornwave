// Exercises src/extension.js against a stub VS Code API.
//
// It cannot prove the extension works in a real host, but it does prove the
// glue: that a provider is registered per language, that an unchanged buffer
// produces no edit, that a changed one produces exactly one whole-document
// replacement, that every failure path returns an empty edit list so the
// buffer is left alone, and that the language server starts only under the
// conditions policy:editor-tool-execution allows a process at all.

import assert from "node:assert/strict";
import Module from "node:module";
import { createRequire } from "node:module";
import { test } from "node:test";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const extensionRoot = join(here, "..");

/** Minimal stand-ins for the VS Code API surface extension.js touches. */
function createStubVscode({ trusted = true, settings = {}, folders = [], confirmAnswer = undefined, uiKind = 1 } = {}) {
  const state = {
    output: [],
    warnings: [],
    providers: [],
    commands: {},
    started: [],
    watched: [],
    terminals: [],
    tasks: [],
    contentProviders: {},
    treeProviders: {},
    opened: [],
    externals: [],
    diagnostics: new Map(),
    taskProviders: [],
    spawned: [],
  };

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
      showWarningMessage: (message, ...rest) => {
        state.warnings.push(message);
        // A modal confirmation answers with its action, so a test can drive a
        // command that asks before it runs. Declining is the default, because
        // a test that forgot to say would otherwise run a migration.
        return Promise.resolve(confirmAnswer);
      },
      createTerminal: (options) => {
        const terminal = {
          options,
          sent: [],
          shown: 0,
          sendText(text) { this.sent.push(text); },
          show() { this.shown += 1; },
          dispose() {},
        };
        state.terminals.push(terminal);
        return terminal;
      },
      onDidCloseTerminal: () => ({ dispose() {} }),
      registerTreeDataProvider: (id, provider) => {
        state.treeProviders[id] = provider;
        return { dispose() {} };
      },
      showTextDocument: () => Promise.resolve({}),
      activeTextEditor: null,
    },
    languages: {
      registerDocumentFormattingEditProvider: (language, provider) => {
        state.providers.push({ language, provider });
        return { dispose() {} };
      },
      setTextDocumentLanguage: (document, language) => {
        document.languageId = language;
        return Promise.resolve(document);
      },
      createDiagnosticCollection: () => ({
        set: (uri, entries) => state.diagnostics.set(uri.fsPath, entries),
        clear: () => state.diagnostics.clear(),
        dispose() {},
      }),
    },
    commands: {
      registerCommand: (command, handler) => {
        state.commands[command] = handler;
        return { dispose() {} };
      },
      executeCommand: (command, ...args) => state.commands[command]?.(...args),
    },
    tasks: {
      registerTaskProvider: (type, provider) => {
        state.taskProviders.push({ type, provider });
        return { dispose() {} };
      },
      executeTask: (task) => {
        state.tasks.push(task);
        return Promise.resolve({ terminate() {} });
      },
    },
    Task: class {
      constructor(definition, scope, name, source, execution, matchers) {
        Object.assign(this, { definition, scope, name, source, execution, matchers });
      }
    },
    TaskScope: { Workspace: 1 },
    ShellExecution: class {
      constructor(command, args, options) {
        Object.assign(this, { command, args, options });
      }
    },
    Diagnostic: class {
      constructor(range, message, severity) {
        Object.assign(this, { range, message, severity });
      }
    },
    Uri: {
      file: (path) => ({ scheme: "file", fsPath: path, toString: () => `file://${path}` }),
      parse: (value) => ({ value, toString: () => value }),
    },
    EventEmitter: class {
      constructor() {
        this.listeners = [];
        this.event = (listener) => {
          this.listeners.push(listener);
          return { dispose() {} };
        };
      }
      fire(value) {
        for (const listener of this.listeners) listener(value);
      }
      dispose() {}
    },
    ViewColumn: { Beside: 2 },
    UIKind: { Desktop: 1, Web: 2 },
    TreeItem: class {
      constructor(label) {
        this.label = label;
      }
    },
    env: {
      uiKind: uiKind,
      openExternal: (uri) => {
        state.externals.push(uri.value ?? uri);
        return Promise.resolve(true);
      },
    },
    workspace: {
      isTrusted: trusted,
      registerTextDocumentContentProvider: (scheme, provider) => {
        state.contentProviders[scheme] = provider;
        return { dispose() {} };
      },
      openTextDocument: (uri) => {
        const document = { uri, languageId: "go" };
        state.opened.push({ uri, document });
        return Promise.resolve(document);
      },
      createFileSystemWatcher: (glob) => {
        state.watched.push(glob);
        return { dispose() {} };
      },
      workspaceFolders: folders.map((path) => ({ uri: { scheme: "file", fsPath: path } })),
      getConfiguration: () => ({
        get: (name, fallback) => (name in settings ? settings[name] : fallback),
      }),
      onDidGrantWorkspaceTrust: () => ({ dispose() {} }),
      onDidChangeConfiguration: () => ({ dispose() {} }),
    },
  };

  return { vscode, state };
}

/**
 * A stand-in for the language client. The real one spawns pw, which is the
 * one thing these tests must not do; what is worth checking is whether the
 * extension decided to start it at all.
 */
function createStubClientModule(state, { failWith = null } = {}) {
  return {
    TransportKind: { stdio: 0 },
    LanguageClient: class {
      constructor(id, name, server, client) {
        this.server = server;
        this.client = client;
      }
      async start() {
        if (failWith) {
          throw new Error(failWith);
        }
        state.started.push(this.server.run);
      }
      async stop() {}
    },
  };
}

/**
 * A child_process whose execFile answers from a script instead of running
 * anything. Both the pw commands and the delegated formatter spawn, and a test
 * that ran either would be testing this machine's pw.
 */
function createStubChildProcess(state, spawnReply) {
  return {
    execFile(file, args, options, callback) {
      const call = { file, args, options, input: "" };
      state.spawned.push(call);
      const finish = () => {
        const reply = spawnReply
          ? spawnReply(call)
          : { code: 0, stdout: "", stderr: "" };
        const error = reply.code === 0 ? null : Object.assign(new Error("exit"), { code: reply.code });
        setImmediate(() => callback(error, reply.stdout ?? "", reply.stderr ?? ""));
      };
      return {
        stdin: {
          end(input) {
            call.input = input ?? "";
            finish();
          },
        },
        // A caller that never writes stdin still expects an answer.
        then: undefined,
      };
    },
  };
}

function loadExtension(vscode, clientModule, childProcess) {
  const require = createRequire(import.meta.url);
  const original = Module._load;
  Module._load = function (request, parent, isMain) {
    if (request === "vscode") {
      return vscode;
    }
    if (request === "vscode-languageclient/node") {
      return clientModule;
    }
    if (request === "node:child_process") {
      return childProcess;
    }
    return original.call(this, request, parent, isMain);
  };
  try {
    // Every module under src/ is reloaded, not only the entry: one that holds
    // the vscode object at require time would otherwise keep the first test's
    // stub and register into a state nobody is looking at.
    for (const cached of Object.keys(require.cache)) {
      if (cached.includes(join(extensionRoot, "src"))) {
        delete require.cache[cached];
      }
    }
    return require(join(extensionRoot, "src", "extension.js"));
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

function activateStub(options = {}) {
  const { vscode, state } = createStubVscode(options);
  const extension = loadExtension(
    vscode,
    createStubClientModule(state, options),
    createStubChildProcess(state, options.spawnReply),
  );
  const context = { extensionPath: extensionRoot, subscriptions: [] };
  extension.activate(context);
  return { state, context };
}

/** Lets the activation's own start() settle before the assertions run. */
const settled = () => new Promise((resolve) => setImmediate(resolve));

test("a formatting provider is registered for each dialect", () => {
  const { state } = activateStub();
  assert.deepEqual(
    state.providers.map((p) => p.language).sort(),
    ["pw-dynamo", "pw-html", "pw-sql"],
  );
  for (const command of ["popcornweb.showOutput", "popcornweb.restartLanguageServer"]) {
    assert.ok(command in state.commands, `${command} is not registered`);
  }
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

test("the language server starts in a trusted workspace with pw on PATH", async () => {
  const { state } = activateStub({
    trusted: true,
    folders: [extensionRoot],
    settings: { "pw.path": process.execPath },
  });
  await settled();

  assert.equal(state.started.length, 1, "the client was not started");
  assert.deepEqual(state.started[0].args.slice(0, 2), ["lsp", "--stdio"]);
  assert.ok(
    state.started[0].args.includes(`--root=${extensionRoot}`),
    "the workspace root was not passed",
  );
});

test("an untrusted workspace starts no process and says what that costs", async () => {
  // policy:editor-tool-execution: opening a file must never run project code,
  // and a workspace-relative binary path is a workspace-controlled input.
  const { state } = activateStub({ trusted: false, settings: { "pw.path": process.execPath } });
  await settled();

  assert.deepEqual(state.started, []);
  const said = state.output.join("\n");
  assert.match(said, /not trusted/);
  assert.match(said, /Highlighting and formatting work/);
});

test("the server can be switched off without losing the formatter", async () => {
  const { state } = activateStub({
    settings: { "languageServer.enabled": false, "pw.path": process.execPath },
  });
  await settled();

  assert.deepEqual(state.started, []);
  assert.match(state.output.join("\n"), /disabled by popcornweb\.languageServer\.enabled/);
  assert.equal(state.providers.length, 3, "formatting is not affected by the server setting");
});

test("formatting is registered whatever the server does", async () => {
  // The stage-1 promise: a developer with no pw, or with one the server
  // cannot start, still gets highlighting and formatting. What resolution
  // reports when it finds nothing is checked in binary.test.mjs, where the
  // lookup can be driven without depending on this machine's PATH.
  const { state } = activateStub({ folders: ["/nowhere"], settings: { "pw.path": "" } });
  await settled();

  assert.equal(state.providers.length, 3);
});

test("a server that fails to start reports why and leaves the rest working", async () => {
  const { state } = activateStub({
    settings: { "pw.path": process.execPath },
    failWith: "spawn ENOENT",
  });
  await settled();

  assert.deepEqual(state.started, []);
  assert.match(state.output.join("\n"), /did not start: spawn ENOENT/);
  assert.equal(state.providers.length, 3);
});

test("every pw command is registered and resolvable as a task", async () => {
  const { state } = activateStub({ folders: [extensionRoot], settings: { "pw.path": process.execPath } });
  await settled();

  for (const id of ["generate", "check", "doctor", "migrate", "dev"]) {
    assert.ok(`popcornweb.${id}` in state.commands, `popcornweb.${id} is not registered`);
  }
  assert.equal(state.taskProviders.length, 1);
  const provided = state.taskProviders[0].provider.provideTasks();
  assert.deepEqual(
    provided.map((task) => task.definition.command).sort(),
    ["check", "doctor", "generate", "migrate"],
  );
});

test("a task runs the resolved binary with the command's own arguments", async () => {
  const { state } = activateStub({ folders: [extensionRoot], settings: { "pw.path": process.execPath } });
  await settled();

  await state.commands["popcornweb.check"]();

  assert.equal(state.tasks.length, 1);
  assert.deepEqual(state.tasks[0].execution.args, ["check"]);
  assert.equal(state.tasks[0].execution.options.cwd, extensionRoot);
  assert.deepEqual(state.tasks[0].matchers, ["$pw", "$pw-source"]);
});

test("pw dev gets one terminal, and a second invocation focuses it", async () => {
  // The loop owns services and the identity provider, so a second one would be
  // a second set of them.
  const { state } = activateStub({ folders: [extensionRoot], settings: { "pw.path": process.execPath } });
  await settled();

  await state.commands["popcornweb.dev"]();
  await state.commands["popcornweb.dev"]();

  assert.equal(state.terminals.length, 1, "a second loop was started");
  assert.equal(state.terminals[0].shown, 2, "the running terminal was not focused");
  assert.deepEqual(state.tasks, [], "pw dev must not run as a task");
});

test("migrate does nothing when the confirmation is declined", async () => {
  // policy:migration-safety: forward-only against a real database, so a
  // dismissed dialog has to mean nothing happened.
  const { state } = activateStub({
    folders: [extensionRoot],
    settings: { "pw.path": process.execPath },
    confirmAnswer: undefined,
  });
  await settled();

  await state.commands["popcornweb.migrate"]();

  assert.deepEqual(state.tasks, []);
  assert.equal(state.warnings.length, 1, "the confirmation was not asked");
});

test("migrate runs once the confirmation is accepted", async () => {
  const { state } = activateStub({
    folders: [extensionRoot],
    settings: { "pw.path": process.execPath },
    confirmAnswer: "Run",
  });
  await settled();

  await state.commands["popcornweb.migrate"]();

  assert.equal(state.tasks.length, 1);
  assert.deepEqual(state.tasks[0].execution.args, ["migrate", "up"]);
});

test("no command starts a process in an untrusted workspace", async () => {
  const { state } = activateStub({
    trusted: false,
    folders: [extensionRoot],
    settings: { "pw.path": process.execPath },
    confirmAnswer: "Run",
  });
  await settled();

  await state.commands["popcornweb.generate"]();
  await state.commands["popcornweb.dev"]();

  assert.deepEqual(state.tasks, []);
  assert.deepEqual(state.terminals, []);
  assert.match(state.output.join("\n"), /not trusted/);
});

test("a command with no folder open says so instead of running from nowhere", async () => {
  // pw finds the project by walking up from where it runs, so a command
  // started from nowhere would find nothing and say something confusing.
  const { state } = activateStub({ folders: [], settings: { "pw.path": process.execPath } });
  await settled();

  await state.commands["popcornweb.check"]();

  assert.deepEqual(state.tasks, []);
  assert.match(state.output.join("\n"), /No folder is open/);
});

test("the generated view and the route tree are registered", async () => {
  const { state } = activateStub({ folders: [extensionRoot], settings: { "pw.path": process.execPath } });
  await settled();

  assert.ok("popcornweb-generated" in state.contentProviders, "the read-only scheme is not registered");
  assert.ok("popcornweb.routes" in state.treeProviders, "the route view is not registered");
  for (const id of ["peekGenerated", "previewStory", "refreshRoutes"]) {
    assert.ok(`popcornweb.${id}` in state.commands, `popcornweb.${id} is not registered`);
  }
});

test("the route tree says the server is not running rather than showing nothing", async () => {
  // An empty tree and a tree that could not be built are different facts, and
  // only one of them means the project has no routes.
  const { state } = activateStub({ folders: [extensionRoot], settings: { "languageServer.enabled": false } });
  await settled();

  const children = await state.treeProviders["popcornweb.routes"].getChildren();
  assert.equal(children.length, 1);
  assert.match(children[0].placeholder, /not running/);
});

test("runtime diagnostics contact nothing while they are off", async () => {
  // The default. policy:editor-tool-execution says the extension contacts
  // nothing, and this is the one feature that would — only once switched on.
  const fetched = [];
  const original = globalThis.fetch;
  globalThis.fetch = (...args) => {
    fetched.push(args);
    return Promise.reject(new Error("no"));
  };
  try {
    activateStub({ folders: [extensionRoot], settings: { "pw.path": process.execPath } });
    await settled();
    await new Promise((resolve) => setTimeout(resolve, 30));
    assert.deepEqual(fetched, []);
  } finally {
    globalThis.fetch = original;
  }
});

test("a browser-hosted editor is told what it does not have", async () => {
  // requirement:editor-web-host: no feature silently fails. An extension that
  // simply does less reads as a broken language server.
  const { state } = activateStub({ uiKind: 2, folders: [], settings: {} });
  await settled();

  const said = state.output.join("\n");
  assert.match(said, /browser-hosted/);
  assert.match(said, /Highlighting and formatting work/);
  assert.match(said, /a web host runs no process/);
});

test("a desktop host says nothing about being a web host", async () => {
  const { state } = activateStub({ folders: [extensionRoot], settings: { "pw.path": process.execPath } });
  await settled();

  assert.doesNotMatch(state.output.join("\n"), /browser-hosted/);
});

/** A pw that formats by upper-casing, and is stable on a second pass. */
const workingPw = (call) => ({ code: 0, stdout: call.input.toUpperCase(), stderr: "" });

test("a trusted workspace with a working pw formats through it", async () => {
  // decision:formatter-delivery prefers this path, because the bytes then come
  // from the same binary the project's own pw fmt runs.
  const { state } = activateStub({
    folders: [extensionRoot],
    settings: { "pw.path": process.execPath },
    spawnReply: workingPw,
  });
  await settled();
  const { provider } = state.providers.find((p) => p.language === "pw-sql");

  const edits = await provider.provideDocumentFormattingEdits(makeDocument("pw-sql", "abc"));

  assert.equal(edits.length, 1);
  assert.equal(edits[0].newText, "ABC");
  const formatCalls = state.spawned.filter((call) => call.args[0] === "fmt");
  assert.equal(formatCalls.length, 3, "two probe passes and the format itself");
  assert.match(state.output.join("\n"), /so the editor and your build agree by construction/);
});

test("a pw whose second pass disagrees is refused and the module is used", async () => {
  // The idempotence guard requirement:editor-formatting relies on. A pw
  // without it would let the editor apply an unstable result.
  let pass = 0;
  const { state } = activateStub({
    folders: [extensionRoot],
    settings: { "pw.path": process.execPath },
    spawnReply: () => ({ code: 0, stdout: `pass ${++pass}\n`, stderr: "" }),
  });
  await settled();
  const { provider } = state.providers.find((p) => p.language === "pw-sql");

  const edits = await provider.provideDocumentFormattingEdits(
    makeDocument("pw-sql", "package q\nexport statement F():sql.exec{DELETE FROM t}\n"),
  );

  // The embedded module produced this, so it is the canonical form rather than
  // the fake pw's answer.
  assert.match(edits[0].newText, /export statement F\(\): sql\.exec \{/);
  assert.match(state.output.join("\n"), /embedded tinybind formatter, because .*second pass/);
});

test("an untrusted workspace formats without starting anything", async () => {
  // The property stage 1 bought and this must not spend: formatting works in a
  // workspace where no process may run.
  const { state } = activateStub({
    trusted: false,
    folders: [extensionRoot],
    settings: { "pw.path": process.execPath },
    spawnReply: workingPw,
  });
  await settled();
  const { provider } = state.providers.find((p) => p.language === "pw-sql");

  const edits = await provider.provideDocumentFormattingEdits(
    makeDocument("pw-sql", "package q\nexport statement F():sql.exec{DELETE FROM t}\n"),
  );

  assert.match(edits[0].newText, /export statement F\(\): sql\.exec \{/);
  assert.deepEqual(state.spawned, []);
  assert.match(state.output.join("\n"), /not trusted/);
});

test("a refusal from pw leaves the buffer alone and names the line", async () => {
  const { state } = activateStub({
    folders: [extensionRoot],
    settings: { "pw.path": process.execPath },
    spawnReply: (call) =>
      call.input.includes("ProbeStyleBraces")
        ? { code: 0, stdout: call.input, stderr: "" }
        : { code: 1, stdout: "", stderr: "pw: fmt: <stdin>:3:1: missing closing tag </p>\n" },
  });
  await settled();
  const { provider } = state.providers.find((p) => p.language === "pw-html");

  const edits = await provider.provideDocumentFormattingEdits(
    makeDocument("pw-html", "export component X(): html {\n<p>unclosed\n", "/tmp/x.pw.html"),
  );

  assert.deepEqual(edits, []);
  assert.equal(state.warnings.length, 1);
  assert.match(state.warnings[0], /was not formatted \(line 3\)/);
});
