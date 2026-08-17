#!/usr/bin/env node
// Self-test for check_docs.mjs.
//
//   node .claude/skills/docs-quality/tests/run_tests.mjs
//
// Two halves. The fixture under tests/fixture/ is a miniature site with one
// planted defect per check, and every check has to find its own. The real site
// is then swept for link errors, which guards the part of the checker most
// likely to rot: the slugger has to agree with github-slugger, and a silent
// disagreement would turn 119 pages of working anchors into false alarms.

import { execFileSync } from 'node:child_process';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = dirname(fileURLToPath(import.meta.url));
const SKILL = join(HERE, '..');
const REPO = join(SKILL, '..', '..', '..');
const CHECKER = join(SKILL, 'check_docs.mjs');

function run(root) {
  const out = execFileSync('node', [CHECKER, '--json'], {
    cwd: REPO,
    encoding: 'utf8',
    env: { ...process.env, PW_REPO_ROOT: root },
    stdio: ['ignore', 'pipe', 'inherit'],
    // A non-empty error list exits 1; that is the expected case here.
  });
  return JSON.parse(out).findings;
}

function runAllowingFailure(root) {
  try {
    return run(root);
  } catch (e) {
    return JSON.parse(e.stdout).findings;
  }
}

// check, severity, a substring of the message, and the file it must land on.
const EXPECTED = [
  ['links', 'error', 'broken internal link', 'guides/frontend/broken.md'],
  ['links', 'error', 'retired route', 'guides/frontend/broken.md'],
  ['links', 'error', 'has no such heading', 'guides/frontend/broken.md'],
  ['links', 'warn', 'links to the English', 'ja/guides/frontend/broken.md'],
  ['links', 'warn', 'English page links into', 'guides/frontend/broken.md'],
  ['frontmatter', 'error', 'no `description`', 'guides/frontend/broken.md'],
  ['frontmatter', 'warn', 'Japanese page says', 'guides/frontend/broken.md'],
  ['parity', 'error', 'no Japanese counterpart', 'guides/frontend/prose.md'],
  ['parity', 'warn', 'top-level sections here', 'reference/runtime.md'],
  ['tailwind', 'error', 'Tailwind utilities', 'tutorial/getting-started.md'],
  ['tailwind', 'error', 'Tailwind utilities', 'ja/tutorial/getting-started.md'],
  ['tutorial', 'info', 'no longer shows', 'tutorial/forms.md'],
  ['sidebar', 'error', 'is in no sidebar group', 'orphan/stray.md'],
  ['sidebar', 'error', 'does not exist', 'astro.config.mjs'],
  ['sidebar', 'error', 'which has no file', 'astro.config.mjs'],
  ['terms', 'error', 'legacy', 'guides/frontend/broken.md'],
  ['terms', 'error', 'wrapper', 'guides/frontend/broken.md'],
  ['terms', 'error', 'template engine', 'guides/frontend/broken.md'],
  ['terms', 'error', '_pw_gen.go', 'guides/frontend/broken.md'],
  ['terms', 'error', 'badge: advanced', 'tutorial/getting-started.md'],
  ['shape', 'warn', 'bullets or table rows', 'guides/frontend/broken.md'],
  ['shape', 'info', 'when *not* to', 'guides/frontend/broken.md'],
];

// Pages the fixture keeps clean, to catch a check that fires on everything.
const MUST_STAY_QUIET = [
  ['guides/frontend/prose.md', 'shape'],
  // Its one link points at a heading spelled `_operation`. Dropping the
  // underscore — which github-slugger keeps — would report that anchor as
  // missing, and that false positive is what this entry guards.
  ['guides/frontend/prose.md', 'links'],
  ['guides/frontend/templates.md', 'links'],
  ['guides/frontend/templates.md', 'terms'],
  // It is the first of two explicit sidebar slugs. A parser that stopped at the
  // first entry would still pass; one that stopped at the second would report
  // this page as belonging to no group.
  ['guides/frontend/templates.md', 'sidebar'],
];

let failed = 0;
const pass = (msg) => console.log(`  ok    ${msg}`);
const fail = (msg) => {
  failed++;
  console.log(`  FAIL  ${msg}`);
};

console.log('fixture: every check finds its planted defect');
const fixture = runAllowingFailure(join(SKILL, 'tests', 'fixture'));
for (const [check, severity, needle, file] of EXPECTED) {
  const hit = fixture.some(
    (f) =>
      f.check === check &&
      f.severity === severity &&
      f.message.includes(needle) &&
      f.file.endsWith(file),
  );
  (hit ? pass : fail)(`${check}/${severity} · ${needle} · ${file}`);
}

console.log('\nfixture: clean pages stay clean');
for (const [file, check] of MUST_STAY_QUIET) {
  const noise = fixture.filter((f) => f.file.endsWith(file) && f.check === check);
  if (noise.length === 0) pass(`${check} silent on ${file}`);
  else fail(`${check} fired on ${file}: ${noise.map((n) => n.message).join('; ')}`);
}

console.log('\nreal site: anchors and internal links resolve');
const real = runAllowingFailure(REPO);
const linkErrors = real.filter((f) => f.check === 'links' && f.severity === 'error');
if (linkErrors.length === 0) pass(`no link errors across the site`);
else {
  fail(`${linkErrors.length} link error(s) — either the docs broke or the slugger drifted from github-slugger`);
  for (const e of linkErrors.slice(0, 5)) console.log(`        ${e.file}:${e.line} ${e.message}`);
}

console.log(`\n${failed === 0 ? 'all checks pass' : `${failed} failing`}`);
process.exit(failed === 0 ? 0 : 1);
