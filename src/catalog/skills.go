package catalog

func skill(name, description, repo, skillName string) App {
	return App{Name: name, Description: description, Category: CategoryAgentSkills, Installs: Anywhere(Skill(repo, skillName))}
}

// invariant: a skill tool is named after its skill, except where that name is
// already a tool's: then it carries the `-skill` suffix, and its install keeps the
// upstream name the skills lock records.

// value: the agents a skill is installed for: Claude Code (~/.claude/skills) and
// the shared ~/.agents/skills that codex and the other universal agents read.
// Without it the skills CLI, finding no agent on a fresh home, writes to all 79.

var SkillAgents = []string{"claude-code", "codex"}

var AgentSkills = []App{
	skill("frontend-design", "Distinctive, polished frontend interfaces", "anthropics/skills", "frontend-design"),
	skill("upgrading-expo", "Upgrade an Expo app across SDK versions", "expo/skills", "upgrading-expo"),
	skill("modern-web-guidance", "Chrome's current web platform guidance", "GoogleChrome/modern-web-guidance", "modern-web-guidance"),
	skill("chrome-extensions", "Chrome extension authoring", "GoogleChrome/modern-web-guidance", "chrome-extensions"),
	skill("better-interface", "Interface polish", "jakubkrehel/skills", "better-interface"),
	skill("better-accessibility", "Accessibility review and fixes", "jakubkrehel/skills", "better-accessibility"),
	skill("better-layout", "Layout and spacing craft", "jakubkrehel/skills", "better-layout"),
	skill("better-writing", "Interface copy", "jakubkrehel/skills", "better-writing"),
	skill("better-typography", "Typography craft", "jakubkrehel/skills", "better-typography"),
	skill("better-colors", "Color craft", "jakubkrehel/skills", "better-colors"),
	skill("better-ui", "UI review across the better-* skills", "jakubkrehel/skills", "better-ui"),
	skill("shadcn", "shadcn/ui components and patterns", "shadcn-ui/ui", "shadcn"),
	skill("last30days", "Research what happened in the last 30 days", "sickn33/antigravity-awesome-skills", "last30days"),
	skill("qmd-skill", "Drive qmd, the local markdown search", "tobi/qmd", "qmd"),
	skill("agent-browser-skill", "Drive agent-browser, the browser automation CLI", "vercel-labs/agent-browser", "agent-browser"),
	skill("vercel-react-best-practices", "React and Next.js performance practices", "vercel-labs/agent-skills", "vercel-react-best-practices"),
	skill("vercel-react-native-skills", "React Native practices", "vercel-labs/agent-skills", "vercel-react-native-skills"),
}
