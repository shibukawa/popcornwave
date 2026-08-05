package pwcheck

import (
	"fmt"
	"strings"
)

// groupTitles name the catalog sections on the reference page, in the order the
// identifier ranges run.
var groupTitles = []struct {
	group string
	title string
	intro string
	// titleJA and introJA carry the same section in Japanese. They live beside
	// the English ones because a section added without them would produce a
	// Japanese page with an English heading, which reads as a translation
	// nobody finished rather than as a page that is generated.
	titleJA string
	introJA string
}{
	{GroupProject, "Project and toolchain (PW01xx)",
		"Whether the project's declared shape, its toolchain, and its generated artifacts still agree with each other.",
		"プロジェクトとツールチェイン (PW01xx)",
		"プロジェクトが宣言した形、ツールチェイン、生成物の三つが、まだ互いに一致しているかどうか。"},
	{GroupRoutes, "Routes and templates (PW02xx)",
		"What the route table says about paths that collide and paths nothing serves. These need a route table `pw generate` does not export yet, so `pw doctor` reports them as not examined.",
		"ルートとテンプレート (PW02xx)",
		"衝突するパスと、誰も応答しないパスについてルート表が語ること。これらには `pw generate` がまだ出力しないルート表が必要なので、`pw doctor` は「検査していない」と報告します。"},
	{GroupStorage, "Storage (PW03xx)",
		"Whether the migration sources are well-formed, and, under `--online`, whether the database still matches them.",
		"ストレージ (PW03xx)",
		"マイグレーションのソースが妥当かどうか。`--online` ではさらに、データベースがまだそれと一致しているかどうか。"},
	{GroupConfig, "Configuration, secrets, and the identity provider (PW04xx)",
		"Three things go wrong here: wiring the binary does not actually carry, values that are inadvisable for the diagnosed environment, and secrets kept in the wrong place.",
		"設定・シークレット・認証プロバイダ (PW04xx)",
		"ここで壊れるものは三つあります。バイナリが実際には持っていない配線、診断対象の環境には勧められない値、そして置き場所を間違えたシークレットです。"},
	{GroupReadiness, "Production readiness (PW05xx)",
		"The pre-launch checklist as something that runs. Silent while the diagnosed environment is `dev`.",
		"本番前チェック (PW05xx)",
		"公開前のチェックリストを、読むものではなく走らせるものにしたもの。診断対象が `dev` のあいだは何も言いません。"},
}

func scopeTextJA(scope Scope) string {
	switch scope {
	case Deployed:
		return "`dev` 以外のすべての環境"
	case Development:
		return "`dev` のみ"
	case Production:
		return "`prod` のみ"
	default:
		return "すべての環境"
	}
}

func severityTextJA(check Check) string {
	if check.Severity == check.DevSeverity {
		return check.Severity.String()
	}
	return fmt.Sprintf("%s（`dev` では %s）", check.Severity, check.DevSeverity)
}

func inputTextJA(inputs Input) string {
	named := []struct {
		input Input
		name  string
	}{
		{Config, "マージ済み設定"},
		{ImportGraph, "import グラフ"},
		{ProjectFiles, "プロジェクトのファイル"},
		{RouteTable, "ルート表"},
		{ProcessEnv, "プロセス環境変数"},
		{Network, "ネットワーク"},
		{OtherEnvironments, "他環境の設定"},
	}
	var parts []string
	for _, entry := range named {
		if inputs&entry.input != 0 {
			parts = append(parts, entry.name)
		}
	}
	return strings.Join(parts, "、")
}

func scopeText(scope Scope) string {
	switch scope {
	case Deployed:
		return "every environment except `dev`"
	case Development:
		return "`dev` only"
	case Production:
		return "`prod` only"
	default:
		return "every environment"
	}
}

func severityText(check Check) string {
	if check.Severity == check.DevSeverity {
		return check.Severity.String()
	}
	return fmt.Sprintf("%s, and %s in `dev`", check.Severity, check.DevSeverity)
}

func inputText(inputs Input) string {
	named := []struct {
		input Input
		name  string
	}{
		{Config, "merged configuration"},
		{ImportGraph, "import graph"},
		{ProjectFiles, "project files"},
		{RouteTable, "route table"},
		{ProcessEnv, "process environment"},
		{Network, "network"},
		{OtherEnvironments, "other environments' configuration"},
	}
	var parts []string
	for _, entry := range named {
		if inputs&entry.input != 0 {
			parts = append(parts, entry.name)
		}
	}
	return strings.Join(parts, ", ")
}

// Markdown renders the diagnostics reference page from the catalog. The page is
// generated rather than written, so an identifier a report prints always has
// something to link to, and a check that is added without documentation fails
// the test that compares this output with the checked-in page.
func Markdown() string {
	var out strings.Builder
	out.WriteString(`---
title: Diagnostics
description: Every pw doctor finding, what it means, and how to resolve it.
sidebar:
  order: 2
---

` + "`pw doctor`" + ` reports one finding per condition, each carrying a stable
identifier such as ` + "`PW0412`" + `. The identifier never changes and is never
reused, so it can be searched, cited in an issue, and looked up here.

Severity is a function of the environment being diagnosed and nothing else:
` + "`pw doctor --env=prod`" + ` judges the same file more strictly than
` + "`pw doctor --env=dev`" + ` does. A check whose scope excludes the diagnosed
environment stays silent rather than being softened.

This page is generated from the check catalog. Adding a check adds its entry
here.
`)
	for _, section := range groupTitles {
		out.WriteString("\n## " + section.title + "\n\n" + section.intro + "\n")
		for _, check := range All() {
			if check.Group != section.group {
				continue
			}
			out.WriteString("\n### " + check.ID + ": " + check.Title + "\n\n")
			out.WriteString("- **Severity**: " + severityText(check) + "\n")
			out.WriteString("- **Applies to**: " + scopeText(check.Scope) + "\n")
			out.WriteString("- **Reads**: " + inputText(check.Inputs) + "\n")
			out.WriteString("- **Fix**: " + check.Remedy + "\n")
		}
	}
	return out.String()
}

// MarkdownJA renders the same page in Japanese.
//
// The identifier, the title, and the remedy stay in English, because those are
// the strings pw doctor prints in the terminal: a reader arrives here with one
// of them in hand, and translating it would break the match they came to make.
// Everything around them is Japanese.
func MarkdownJA() string {
	var out strings.Builder
	out.WriteString(`---
title: 診断
description: pw doctor が報告するすべての項目と、その意味と、直し方。
sidebar:
  order: 2
---

` + "`pw doctor`" + ` は条件ひとつにつき項目ひとつを報告し、それぞれが ` + "`PW0412`" + ` のような
安定した識別子を持ちます。識別子は変わることも再利用されることもないので、検索し、issue に
引用し、ここで引くことができます。

深刻度を決めるのは診断対象の環境だけです。` + "`pw doctor --env=prod`" + ` は
` + "`pw doctor --env=dev`" + ` より同じファイルを厳しく見ます。診断対象の環境を範囲に含まない
チェックは、深刻度が下がるのではなく黙ります。

このページはチェックカタログから生成されています。チェックを追加すれば、その項目もここに
増えます。項目名と直し方が英語のままなのは、それがターミナルに出る文字列そのものだからです。
`)
	for _, section := range groupTitles {
		out.WriteString("\n## " + section.titleJA + "\n\n" + section.introJA + "\n")
		for _, check := range All() {
			if check.Group != section.group {
				continue
			}
			out.WriteString("\n### " + check.ID + ": " + check.Title + "\n\n")
			out.WriteString("- **深刻度**: " + severityTextJA(check) + "\n")
			out.WriteString("- **対象**: " + scopeTextJA(check.Scope) + "\n")
			out.WriteString("- **読むもの**: " + inputTextJA(check.Inputs) + "\n")
			out.WriteString("- **直し方**: " + check.Remedy + "\n")
		}
	}
	return out.String()
}
