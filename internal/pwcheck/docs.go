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
}{
	{GroupProject, "Project and toolchain (PW01xx)",
		"Whether the project's declared shape, its toolchain, and its generated artifacts still agree with each other."},
	{GroupRoutes, "Routes and templates (PW02xx)",
		"What the route table says about paths that collide and paths nothing serves. These need a route table `pw generate` does not export yet, so `pw doctor` reports them as not examined."},
	{GroupStorage, "Storage (PW03xx)",
		"Whether the migration sources are well-formed, and, under `--online`, whether the database still matches them."},
	{GroupConfig, "Configuration, secrets, and the identity provider (PW04xx)",
		"Wiring the binary does not carry, values that are inadvisable for the diagnosed environment, and secrets that are in the wrong place."},
	{GroupReadiness, "Production readiness (PW05xx)",
		"The pre-launch checklist as something that runs. Silent while the diagnosed environment is `dev`."},
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
