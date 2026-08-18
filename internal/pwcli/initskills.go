package pwcli

import (
	"io/fs"
	"path"
	"strings"

	"github.com/shibukawa/popcornweb/skills"
)

// Agent skill answers of pw init. The framework ships a usage guideline an AI
// coding agent loads as a skill — template and query syntax, project anatomy,
// and the pw commands that check an edit — and this answer decides which agent
// directory a copy lands in, if any.
const (
	// skillsClaude places the skill where Claude Code discovers it.
	skillsClaude = "claude"
	// skillsAgents places the same tree under .agents/, the layout shared by
	// other coding agents.
	skillsAgents = "agents"
	skillsNone   = "none"
)

func validSkills(value string) bool {
	return value == skillsClaude || value == skillsAgents || value == skillsNone
}

// skillsRoot is the directory tree an answer places the skill under. The none
// answer places nothing, which is what the empty root means.
func skillsRoot(value string) string {
	switch value {
	case skillsClaude:
		return ".claude"
	case skillsAgents:
		return ".agents"
	}
	return ""
}

// agentSkillDir is the directory the placed skill lives in under
// <root>/skills/. It is the skill's name rather than the repository folder it
// is authored in, because the directory name is how an agent addresses a skill.
const agentSkillDir = "popcornweb"

// embeddedSkillDir is where the same tree lives inside skills.PopcornWeb.
const embeddedSkillDir = "popcornweb-skill"

// agentSkillFiles is the bundled framework skill, keyed by project-relative
// path under the chosen agent directory. A project that declined the skill
// gets nothing.
func agentSkillFiles(options initOptions) map[string]string {
	root := skillsRoot(options.Skills)
	if root == "" {
		return nil
	}
	files := map[string]string{}
	err := fs.WalkDir(skills.PopcornWeb, embeddedSkillDir, func(source string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		content, err := skills.PopcornWeb.ReadFile(source)
		if err != nil {
			return err
		}
		relative := strings.TrimPrefix(source, embeddedSkillDir+"/")
		files[path.Join(root, "skills", agentSkillDir, relative)] = string(content)
		return nil
	})
	if err != nil {
		// The tree is embedded at build time, so a walk that fails means the
		// binary itself is broken rather than anything about this run.
		panic("pw init: cannot read the bundled agent skill: " + err.Error())
	}
	return files
}

// mergeAgentSkillFiles adds the skill to a scaffold. Both project kinds take
// it, because a package project authors the same template and query sources an
// application does.
func mergeAgentSkillFiles(files map[string]string, options initOptions) {
	for path, content := range agentSkillFiles(options) {
		files[path] = content
	}
}
