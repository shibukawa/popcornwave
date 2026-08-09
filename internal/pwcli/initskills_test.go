package pwcli

import (
	"strings"
	"testing"
)

// TestParseInitArgsSkills covers the answer's three values and its rejection.
func TestParseInitArgsSkills(t *testing.T) {
	for _, testcase := range []struct {
		args []string
		want string
	}{
		{args: []string{"demo"}, want: skillsClaude},
		{args: []string{"demo", "--skills=claude"}, want: skillsClaude},
		{args: []string{"demo", "--skills=agents"}, want: skillsAgents},
		{args: []string{"demo", "--skills=none"}, want: skillsNone},
	} {
		options, err := parseInitArgs(testcase.args)
		if err != nil {
			t.Fatal(err)
		}
		if options.Skills != testcase.want {
			t.Fatalf("parseInitArgs(%v).Skills = %q, want %q", testcase.args, options.Skills, testcase.want)
		}
	}
	if _, err := parseInitArgs([]string{"demo", "--skills=cursor"}); err == nil {
		t.Fatal("an unknown agent directory was accepted")
	}
}

// TestScaffoldPlacesAgentSkill asserts that the unset answer and each explicit
// one place the bundled skill where that agent reads, or nowhere.
func TestScaffoldPlacesAgentSkill(t *testing.T) {
	for _, testcase := range []struct {
		name   string
		skills string
		root   string
	}{
		{name: "the default answers place the Claude Code directory", skills: skillsClaude, root: ".claude"},
		{name: "agents", skills: skillsAgents, root: ".agents"},
		{name: "none places nothing", skills: skillsNone, root: ""},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			options := defaultInitOptions()
			options.Name = "demo"
			options.Skills = testcase.skills
			files := scaffoldFiles(options)
			placed := ""
			for path := range files {
				if strings.Contains(path, "skills/"+agentSkillDir+"/SKILL.md") {
					placed = path
				}
			}
			if testcase.root == "" {
				if placed != "" {
					t.Fatalf("declined skill still scaffolded %s", placed)
				}
				return
			}
			want := testcase.root + "/skills/" + agentSkillDir + "/SKILL.md"
			if placed != want {
				t.Fatalf("skill entry point at %q, want %q", placed, want)
			}
			// The entry point names the references beside it, so placing it
			// without them would ship dangling links.
			if !strings.Contains(files[want], "references/templates.md") {
				t.Fatal("SKILL.md does not point at its references")
			}
			reference := testcase.root + "/skills/" + agentSkillDir + "/references/templates.md"
			if _, ok := files[reference]; !ok {
				t.Fatalf("%s is missing", reference)
			}
		})
	}
}

// TestPackageScaffoldPlacesAgentSkill covers the other project kind: a package
// authors the same template and query sources, so it takes the same guideline.
func TestPackageScaffoldPlacesAgentSkill(t *testing.T) {
	options := defaultInitOptions()
	options.Name = "github.com/example/mycomponent"
	options.Kind = kindPackage
	files := scaffoldFiles(options)
	if _, ok := files[".claude/skills/"+agentSkillDir+"/SKILL.md"]; !ok {
		t.Fatal("a package scaffold carries no agent skill")
	}
}

// TestPresetKeepsSkillsAnswer asserts the answer survives a preset the way the
// name does, because no preset answers it.
func TestPresetKeepsSkillsAnswer(t *testing.T) {
	options, err := parseInitArgs([]string{"demo", "--preset=" + presetWebsiteLogin, "--skills=none"})
	if err != nil {
		t.Fatal(err)
	}
	if options.Skills != skillsNone {
		t.Fatalf("preset overwrote the skills answer with %q", options.Skills)
	}
}
