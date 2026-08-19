"use strict";

// The system:pw-cli commands requirement:editor-tasks makes reachable from the
// editor, and the terms each one runs on.
//
// The table is data so the same definition drives a command, a resolvable task,
// and the manifest's own list; a command reachable one way and not the other is
// how the two drift apart.

/**
 * @typedef {object} Command
 * @property {string} id            the popcornweb.<id> command and the task's `command` field
 * @property {string} title         what the palette shows
 * @property {string[]} args        the pw arguments
 * @property {"task"|"terminal"|"report"} kind how the output is handled
 * @property {boolean} writes       whether running it changes files in the workspace
 * @property {string} [confirm]     a modal question that must be answered first
 * @property {string} [detail]      one line of why, shown beside the task
 */

/** @type {Command[]} */
const COMMANDS = [
  {
    id: "generate",
    title: "Generate",
    args: ["generate", "--code-only"],
    kind: "task",
    writes: true,
    detail:
      "Write the generated Go a diagnostic points into. --code-only leaves the asset tree to a build.",
  },
  {
    id: "check",
    title: "Check",
    args: ["check"],
    kind: "task",
    writes: false,
    detail: "Report generated Go that is stale or missing. Writes nothing, so it is safe on save.",
  },
  {
    id: "doctor",
    title: "Doctor",
    args: ["doctor", "--format=json"],
    kind: "report",
    writes: false,
    detail: "Report what an environment would run, as diagnostics on the files it names.",
  },
  {
    id: "migrate",
    title: "Migrate",
    args: ["migrate", "up"],
    kind: "task",
    writes: false,
    // policy:migration-safety makes this forward-only against a real database,
    // so the editor asks before a click reaches it.
    confirm:
      "Apply pending migrations? This runs against the configured database and cannot be rolled back by this command.",
    detail: "Apply pending migrations. Asks first, because it reaches a real database.",
  },
  {
    id: "dev",
    title: "Dev",
    args: ["dev"],
    kind: "terminal",
    writes: true,
    detail:
      "Watch, regenerate, rebuild, and restart. Owns services and a terminal, and is never started implicitly.",
  },
];

/** The problem matchers the extension contributes, in the order a task uses them. */
const PROBLEM_MATCHERS = ["$pw", "$pw-source"];

/** The task type a tasks.json entry names. */
const TASK_TYPE = "pw";

function commandById(id) {
  return COMMANDS.find((command) => command.id === id);
}

/** The commands offered as resolvable tasks. A terminal command is not one:
 * a task's output is captured, and pw dev owns an interactive terminal. */
function taskCommands() {
  return COMMANDS.filter((command) => command.kind !== "terminal");
}

module.exports = { COMMANDS, PROBLEM_MATCHERS, TASK_TYPE, commandById, taskCommands };
