// The acceptance criteria of requirement:template-syntax-highlighting, one
// test each, plus the scope contract of rule:template-grammar-scopes.

import assert from "node:assert/strict";
import { test } from "node:test";

import { allOf, hasScope, scopesOf, tokenize } from "./tokenize.mjs";

test("header keywords carry their fixed scopes", async () => {
  const tokens = await tokenize(
    [
      "package queries",
      'import "shared/types" as types',
      "type Row { count: int }",
      "export statement Find(id: int): sql.one<Row> {",
      "SELECT 1",
      "}",
    ].join("\n"),
    "source.pw.sql",
  );

  assert.ok(hasScope(scopesOf(tokens, "package"), "keyword.control.import.pw"));
  assert.ok(hasScope(scopesOf(tokens, "import"), "keyword.control.import.pw"));
  assert.ok(hasScope(scopesOf(tokens, "as"), "keyword.control.import.pw"));
  assert.ok(hasScope(scopesOf(tokens, "type"), "storage.type.pw"));
  assert.ok(hasScope(scopesOf(tokens, "export"), "storage.modifier.pw"));
  assert.ok(hasScope(scopesOf(tokens, "statement"), "keyword.declaration.pw"));
  assert.ok(hasScope(scopesOf(tokens, "Find"), "entity.name.function.pw"));
  assert.ok(hasScope(scopesOf(tokens, "id"), "variable.parameter.pw"));
  assert.ok(hasScope(scopesOf(tokens, "sql.one"), "support.type.output.pw"));
  assert.ok(hasScope(scopesOf(tokens, "Row"), "entity.name.type.pw"));
});

test("an annotation colors its name and its string arguments", async () => {
  const tokens = await tokenize(
    [
      '@cache(ttl: "5m")',
      "component Badge(tone: string): html {",
      "<span>{tone}</span>",
      "}",
    ].join("\n"),
    "source.pw.html",
  );

  assert.ok(hasScope(scopesOf(tokens, "@cache"), "entity.name.tag.annotation.pw"));
  assert.ok(hasScope(scopesOf(tokens, "ttl"), "variable.parameter.pw"));
  assert.ok(hasScope(scopesOf(tokens, "5m"), "string.quoted.double.pw"));
  assert.ok(hasScope(scopesOf(tokens, "component"), "keyword.declaration.pw"));
});

test("an html body is embedded and its expressions reopen the template scope", async () => {
  const tokens = await tokenize(
    [
      "export component Home(count: int): html {",
      "<strong>{count}</strong>",
      "}",
    ].join("\n"),
    "source.pw.html",
  );

  const strong = allOf(tokens, "strong")[0];
  assert.ok(hasScope(strong.scopes, "meta.embedded.block.html"));
  assert.ok(hasScope(strong.scopes, "entity.name.tag.html"));

  const value = tokens.find((t) => t.text === "count" && t.line === 2);
  assert.ok(hasScope(value.scopes, "meta.embedded.expression.pw"));
  assert.ok(hasScope(value.scopes, "variable.other.pw"));

  const open = tokens.find((t) => t.text === "{" && t.line === 2);
  assert.ok(hasScope(open.scopes, "punctuation.section.embedded.pw"));
});

test("an attribute value keeps its string scope around the expression", async () => {
  const tokens = await tokenize(
    [
      "export component Chip(color: string): html {",
      '<span style="background-color: {color}">x</span>',
      "}",
    ].join("\n"),
    "source.pw.html",
  );

  const literal = tokens.find((t) => t.text.includes("background-color"));
  assert.ok(hasScope(literal.scopes, "string.quoted.double.html"));

  const expression = tokens.find((t) => t.text === "color" && t.line === 2);
  assert.ok(
    hasScope(expression.scopes, "meta.embedded.expression.pw"),
    "the expression inside the attribute is not scoped as an expression",
  );
  assert.ok(
    hasScope(expression.scopes, "string.quoted.double.html"),
    "the attribute string scope is not kept around the expression",
  );
  assert.ok(hasScope(scopesOf(tokens, "style"), "entity.other.attribute-name.html"));
});

test("an unquoted attribute value is a template expression", async () => {
  const tokens = await tokenize(
    [
      "export component Card(user: User): html {",
      "<Badge label={user.name}><em>member</em></Badge>",
      "}",
    ].join("\n"),
    "source.pw.html",
  );

  assert.ok(hasScope(scopesOf(tokens, "Badge"), "entity.name.tag.component.pw"));
  assert.ok(hasScope(scopesOf(tokens, "label"), "entity.other.attribute-name.html"));
  const user = tokens.find((t) => t.text === "user" && t.line === 2);
  assert.ok(hasScope(user.scopes, "meta.embedded.expression.pw"));
  assert.ok(hasScope(scopesOf(tokens, "name"), "variable.other.property.pw"));
});

test("control forms color their keywords and keep the body in html", async () => {
  const tokens = await tokenize(
    [
      "export component List(items: string[]): html {",
      "{for item in items}",
      "  <li>{item}</li>",
      "{/for}",
      "}",
    ].join("\n"),
    "source.pw.html",
  );

  assert.ok(hasScope(scopesOf(tokens, "for"), "keyword.control.pw"));
  assert.ok(hasScope(scopesOf(tokens, "in"), "keyword.control.pw"));
  assert.ok(hasScope(scopesOf(tokens, "/"), "keyword.control.pw"));
  const li = tokens.find((t) => t.text === "li" && t.line === 3);
  assert.ok(hasScope(li.scopes, "meta.embedded.block.html"));
});

test("an await block colors every clause keyword", async () => {
  const tokens = await tokenize(
    [
      "export component Profile(id: string): html {",
      "{await user = LoadUser(id)}",
      "  <p>{user.name}</p>",
      "{fallback}",
      "  <p>waiting</p>",
      "{recover err}",
      "  <p>{err.message}</p>",
      "{/await}",
      "}",
    ].join("\n"),
    "source.pw.html",
  );

  for (const keyword of ["await", "fallback", "recover"]) {
    assert.ok(
      hasScope(scopesOf(tokens, keyword), "keyword.control.pw"),
      `${keyword} is not a control keyword`,
    );
  }
  assert.ok(hasScope(scopesOf(tokens, "LoadUser"), "entity.name.function.pw"));
});

test("a slot is template syntax rather than an html element", async () => {
  const tokens = await tokenize(
    [
      "component Panel(children: html): html {",
      '<div><slot name="header" /></div>',
      "}",
    ].join("\n"),
    "source.pw.html",
  );

  assert.ok(hasScope(scopesOf(tokens, "slot"), "entity.name.tag.slot.pw"));
  assert.ok(hasScope(scopesOf(tokens, "div"), "entity.name.tag.html"));
});

test("{{ }} is literal text and carries no expression", async () => {
  const tokens = await tokenize(
    ["component Doc(): html {", "<p>{{name}}</p>", "}"].join("\n"),
    "source.pw.html",
  );

  const escaped = tokens.filter((t) => t.line === 2 && t.text.includes("name"));
  for (const token of escaped) {
    assert.ok(
      hasScope(token.scopes, "constant.character.escape.pw"),
      "an escaped brace region is not literal",
    );
    assert.ok(
      !hasScope(token.scopes, "meta.embedded.expression.pw"),
      "an escaped brace region was read as an expression",
    );
  }
});

test("a SELECT with a brace inside a string literal is not an expression", async () => {
  const tokens = await tokenize(
    [
      "export statement Find(id: int): sql.one<Row> {",
      "SELECT '{not a placeholder}' AS label FROM t WHERE id = {id}",
      "}",
    ].join("\n"),
    "source.pw.sql",
  );

  const inside = tokens.find((t) => t.text.includes("not a placeholder"));
  assert.ok(
    hasScope(inside.scopes, "string.quoted.single.sql"),
    "the string literal lost its SQL string scope",
  );
  assert.ok(
    !hasScope(inside.scopes, "meta.embedded.expression.pw"),
    "a brace inside a SQL string literal opened an expression",
  );

  const parameter = tokens.filter(
    (t) => t.text === "id" && hasScope(t.scopes, "meta.embedded.expression.pw"),
  );
  assert.equal(parameter.length, 1, "the real placeholder was not recognized");
});

test("SQL comments and dollar quoting hide braces too", async () => {
  const tokens = await tokenize(
    [
      "export statement Odd(): sql.exec {",
      "-- a comment with {braces}",
      "/* a block with {braces} */",
      "SELECT $tag${still {braces}}$tag$",
      "}",
    ].join("\n"),
    "source.pw.sql",
  );

  for (const token of tokens.filter((t) => t.text.includes("braces"))) {
    assert.ok(
      !hasScope(token.scopes, "meta.embedded.expression.pw"),
      `a brace was read as an expression in ${JSON.stringify(token.text)}`,
    );
  }
});

test("a quoted identifier hides braces", async () => {
  const tokens = await tokenize(
    [
      "export statement Weird(): sql.exec {",
      'SELECT "col{1}" FROM t',
      "}",
    ].join("\n"),
    "source.pw.sql",
  );

  const identifier = tokens.find((t) => t.text.includes("col{1}"));
  assert.ok(hasScope(identifier.scopes, "string.quoted.double.sql"));
  assert.ok(!hasScope(identifier.scopes, "meta.embedded.expression.pw"));
});

test("a dynamo body carries no SQL scope anywhere", async () => {
  const tokens = await tokenize(
    [
      "export statement ReadingsSince(sensor: string, from: int64): dynamo.many<Reading> {",
      "  table readings",
      "  key sensor = {sensor} and at > {from}",
      "}",
    ].join("\n"),
    "source.pw.dynamo",
  );

  for (const token of tokens) {
    for (const scope of token.scopes) {
      assert.ok(
        !scope.includes(".sql"),
        `${JSON.stringify(token.text)} carries the SQL scope ${scope}`,
      );
    }
  }

  assert.ok(hasScope(scopesOf(tokens, "dynamo.many"), "support.type.output.pw"));
  assert.ok(hasScope(scopesOf(tokens, "table"), "keyword.other.clause.pw"));
  assert.ok(hasScope(scopesOf(tokens, "readings"), "entity.name.table.pw"));
  assert.ok(hasScope(scopesOf(tokens, "key"), "keyword.other.clause.pw"));
  assert.ok(hasScope(scopesOf(tokens, "and"), "keyword.operator.logical.pw"));
  assert.ok(hasScope(scopesOf(tokens, "at"), "variable.other.attribute.pw"));

  const bound = tokens.filter(
    (t) => t.text === "sensor" && hasScope(t.scopes, "meta.embedded.expression.pw"),
  );
  assert.equal(bound.length, 1, "the key parameter is not an expression");
});

test("only the clauses the dynamo parser has are colored as clauses", async () => {
  // The body grammar is closed: table, key, and the filter it rejects by name.
  // limit, index, and consistent read are driver options at the call site, so
  // coloring them here would advertise syntax that does not exist.
  const tokens = await tokenize(
    [
      "export statement Q(k: string): dynamo.many<R> {",
      "  table t",
      "  key pk = {k}",
      "  limit index consistent forward backward",
      "}",
    ].join("\n"),
    "source.pw.dynamo",
  );

  for (const word of ["limit", "index", "consistent", "forward", "backward"]) {
    assert.ok(
      !hasScope(scopesOf(tokens, word), "keyword.other.clause.pw"),
      `${word} is not a clause but is colored as one`,
    );
  }
  assert.ok(hasScope(scopesOf(tokens, "table"), "keyword.other.clause.pw"));
  assert.ok(hasScope(scopesOf(tokens, "key"), "keyword.other.clause.pw"));
});

test("style content keeps its braces, per the raw text insertion gate", async () => {
  const tokens = await tokenize(
    [
      "component Doc(): html {",
      "<style>",
      ".card { color: red; }",
      "</style>",
      "}",
    ].join("\n"),
    "source.pw.html",
  );

  const cssBraces = tokens.filter((t) => t.line === 3 && /[{}]/.test(t.text));
  assert.ok(cssBraces.length > 0, "no CSS brace was tokenized");
  for (const token of cssBraces) {
    assert.ok(
      !hasScope(token.scopes, "punctuation.section.embedded.pw"),
      "a spaced CSS block brace was read as a template insertion",
    );
  }
});

test("a tight call shape in script content is still an insertion", async () => {
  const tokens = await tokenize(
    [
      "component Doc(payload: Payload): html {",
      "<script>window.payload = {JsonForScript(payload)};</script>",
      "</script>",
      "}",
    ].join("\n"),
    "source.pw.html",
  );

  const call = tokens.find((t) => t.text === "JsonForScript");
  assert.ok(call, "the intrinsic call was not tokenized");
  assert.ok(hasScope(call.scopes, "meta.embedded.expression.pw"));
  assert.ok(
    hasScope(call.scopes, "support.function.builtin.pw"),
    "a compiler-known intrinsic is scoped as an ordinary call",
  );
});

test("an await binding colors its assignment", async () => {
  const tokens = await tokenize(
    [
      "component P(id: string): html {",
      "{await user = LoadUser(id)}",
      "<p>{user.name}</p>",
      "{/await}",
      "}",
    ].join("\n"),
    "source.pw.html",
  );

  const equals = tokens.find((t) => t.line === 2 && t.text.trim() === "=");
  assert.ok(equals, "the binding assignment was not tokenized");
  assert.ok(hasScope(equals.scopes, "keyword.operator.pw"));
});

test("a JavaScript template literal placeholder is not an insertion", async () => {
  const tokens = await tokenize(
    [
      "component Doc(): html {",
      "<script>const s = `hi ${name}`;</script>",
      "}",
    ].join("\n"),
    "source.pw.html",
  );

  // With no JavaScript grammar loaded the whole script body is one token, so
  // the assertion is that no part of it entered the template scope.
  for (const token of tokens.filter((t) => t.line === 2)) {
    assert.ok(
      !hasScope(token.scopes, "meta.embedded.expression.pw"),
      `a \${} placeholder was read as a template insertion in ${JSON.stringify(token.text)}`,
    );
  }
  assert.ok(
    tokens.some(
      (t) => t.line === 2 && t.text.includes("${name}") && hasScope(t.scopes, "meta.embedded.block.js"),
    ),
    "the script body was not treated as embedded JavaScript",
  );
});

test("an unterminated body degrades to text and produces no invalid scope", async () => {
  const tokens = await tokenize(
    ["export component Broken(): html {", "<p>never closed", ""].join("\n"),
    "source.pw.html",
  );

  for (const token of tokens) {
    for (const scope of token.scopes) {
      assert.ok(
        !scope.startsWith("invalid."),
        `${JSON.stringify(token.text)} was marked invalid`,
      );
    }
  }
});

test("a declaration body ends at its own closing brace", async () => {
  const tokens = await tokenize(
    [
      "component First(): html {",
      "<p>one</p>",
      "}",
      "",
      "export component Second(): html {",
      "<p>two</p>",
      "}",
    ].join("\n"),
    "source.pw.html",
  );

  const second = tokens.find((t) => t.text === "Second");
  assert.ok(hasScope(second.scopes, "entity.name.function.pw"));
  assert.ok(
    !hasScope(second.scopes, "meta.embedded.block.html"),
    "the first declaration body swallowed the second declaration",
  );
});
