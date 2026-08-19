"use strict";

// The requirement:editor-tasks half of the extension: the system:pw-cli
// commands, as palette entries and as resolvable tasks.
//
// Every one of them starts a process, so policy:editor-tool-execution governs
// all of it: nothing runs without an explicit action, nothing runs in an
// untrusted workspace, and the binary is the one binary.js resolved.

const vscode = require("vscode");

const { COMMANDS, PROBLEM_MATCHERS, TASK_TYPE, commandById, taskCommands } = require("./tasks");
const { reportDiagnostics, reportLimits } = require("./doctor");

/**
 * Runs the pw commands and owns what they produce.
 *
 * @param {() => {binary: string, folder: string} | null} resolve supplies the
 *   binary and the folder to run in, or null with the reason already reported
 */
class CommandRunner {
  constructor(output, resolve) {
    this.output = output;
    this.resolve = resolve;
    this.devTerminal = null;
    this.diagnostics = vscode.languages.createDiagnosticCollection("popcornweb-doctor");
  }

  /** Registers every command and the task provider. */
  register(context) {
    for (const command of COMMANDS) {
      context.subscriptions.push(
        vscode.commands.registerCommand(`popcornweb.${command.id}`, () => this.run(command.id)),
      );
    }
    context.subscriptions.push(
      this.diagnostics,
      vscode.tasks.registerTaskProvider(TASK_TYPE, {
        provideTasks: () => this.provideTasks(),
        resolveTask: (task) => this.resolveTask(task),
      }),
      vscode.window.onDidCloseTerminal((terminal) => {
        if (terminal === this.devTerminal) {
          this.devTerminal = null;
        }
      }),
    );
  }

  async run(id) {
    const command = commandById(id);
    if (!command) {
      return;
    }
    if (!vscode.workspace.isTrusted) {
      this.output.appendLine(
        `${command.title} was not run: this workspace is not trusted, so no process is started.`,
      );
      this.output.show();
      return;
    }
    const resolved = this.resolve();
    if (!resolved) {
      this.output.show();
      return;
    }
    if (command.confirm) {
      const answer = await vscode.window.showWarningMessage(
        command.confirm,
        { modal: true },
        "Run",
      );
      if (answer !== "Run") {
        return;
      }
    }

    switch (command.kind) {
      case "terminal":
        this.runInTerminal(command, resolved);
        return;
      case "report":
        await this.runDoctor(command, resolved);
        return;
      default:
        await vscode.tasks.executeTask(this.taskFor(command, resolved));
    }
  }

  /**
   * pw dev gets one long-lived terminal of its own, because the loop owns
   * services, the identity provider, and the telemetry viewer. A second
   * invocation focuses the running one rather than starting a second loop.
   *
   * The banner URLs the loop prints are links because the terminal makes them
   * links; the extension embeds no viewer, per requirement:editor-tasks.
   */
  runInTerminal(command, resolved) {
    if (this.devTerminal) {
      this.devTerminal.show();
      return;
    }
    this.devTerminal = vscode.window.createTerminal({
      name: "pw dev",
      cwd: resolved.folder,
    });
    this.devTerminal.sendText(`${quote(resolved.binary)} ${command.args.join(" ")}`);
    this.devTerminal.show();
  }

  /**
   * pw doctor is read as a report rather than as output: its findings name
   * files and configuration lines, and a diagnostic on those is what makes
   * them navigable.
   */
  async runDoctor(command, resolved) {
    this.output.appendLine(`Running ${command.args.join(" ")} in ${resolved.folder}.`);
    const { code, stdout, stderr } = await this.capture(resolved, command.args);
    if (stdout.trim() === "") {
      this.output.appendLine(`pw doctor produced no report (exit ${code}). ${stderr.trim()}`);
      this.output.show();
      return;
    }

    let report;
    try {
      report = JSON.parse(stdout);
    } catch (error) {
      this.output.appendLine(`pw doctor produced no readable report: ${error.message}`);
      this.output.show();
      return;
    }

    this.diagnostics.clear();
    let total = 0;
    for (const [file, findings] of reportDiagnostics(resolved.folder, report)) {
      const uri = vscode.Uri.file(absolute(resolved.folder, file));
      this.diagnostics.set(
        uri,
        findings.map((finding) => {
          const line = Math.max(0, finding.line - 1);
          const diagnostic = new vscode.Diagnostic(
            new vscode.Range(line, 0, line, Number.MAX_SAFE_INTEGER),
            finding.message,
            finding.severity - 1,
          );
          diagnostic.source = "pw doctor";
          diagnostic.code = finding.docs
            ? { value: finding.code, target: vscode.Uri.parse(finding.docs) }
            : finding.code;
          return diagnostic;
        }),
      );
      total += findings.length;
    }

    // A report that looks clean because it did not look is the failure mode
    // the limits exist to prevent, so they are printed whether or not there
    // were findings.
    const limits = reportLimits(report);
    this.output.appendLine(`pw doctor: ${total} finding(s), ${limits.length} thing(s) it could not determine.`);
    for (const limit of limits) {
      this.output.appendLine(`  not determined — ${limit}`);
    }
  }

  provideTasks() {
    const resolved = this.resolve();
    if (!resolved) {
      return [];
    }
    return taskCommands().map((command) => this.taskFor(command, resolved));
  }

  /** Resolves a tasks.json entry that named only its type and command. */
  resolveTask(task) {
    const command = commandById(task.definition?.command);
    const resolved = this.resolve();
    if (!command || !resolved) {
      return undefined;
    }
    const built = this.taskFor(command, resolved, task.definition.args);
    built.definition = task.definition;
    return built;
  }

  taskFor(command, resolved, extraArgs) {
    const args = [...command.args, ...(extraArgs ?? [])];
    const task = new vscode.Task(
      { type: TASK_TYPE, command: command.id },
      vscode.TaskScope.Workspace,
      `pw ${command.id}`,
      "pw",
      new vscode.ShellExecution(quote(resolved.binary), args, { cwd: resolved.folder }),
      PROBLEM_MATCHERS,
    );
    task.detail = command.detail;
    return task;
  }

  /** Runs one command and collects its output, without a terminal. */
  capture(resolved, args) {
    const { execFile } = require("node:child_process");
    return new Promise((resolve) => {
      execFile(
        resolved.binary,
        args,
        { cwd: resolved.folder, maxBuffer: 16 * 1024 * 1024 },
        (error, stdout, stderr) => {
          resolve({ code: error?.code ?? 0, stdout: stdout ?? "", stderr: stderr ?? "" });
        },
      );
    });
  }

  dispose() {
    this.diagnostics.dispose();
  }
}

function absolute(root, file) {
  const { isAbsolute, join } = require("node:path");
  return isAbsolute(file) ? file : join(root, file);
}

/** Quotes a path for a shell execution, which is how a task runs. */
function quote(path) {
  return /[\s"']/.test(path) ? `"${path}"` : path;
}

module.exports = { CommandRunner };
