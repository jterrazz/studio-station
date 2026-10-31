package catalog

type GitShortcut struct {
	Name, Description string
	MinArgs           int
	Steps             []GitStep
}

// invariant: AppendArgs, when set, is the prefix of ONE more argument made of the
// shortcut's own words joined by spaces (`feat: <message>`).

type GitStep struct {
	Args          []string
	AppendArgs    string
	IgnoreFailure bool
}

type DockerShortcut struct{ Name, Description string }

var GitShortcuts = []GitShortcut{
	{Name: "branches", Description: "List local branches", Steps: []GitStep{{Args: []string{"branch"}}}},
	{Name: "chore", Description: "Add all and commit with the chore: prefix", MinArgs: 1, Steps: commit("chore: ")},
	{Name: "feat", Description: "Add all and commit with the feat: prefix", MinArgs: 1, Steps: commit("feat: ")},
	{Name: "fix", Description: "Add all and commit with the fix: prefix", MinArgs: 1, Steps: commit("fix: ")},
	{Name: "log", Description: "Show the ten last commits", Steps: []GitStep{{Args: []string{"log", "--oneline", "-10"}}}},
	{Name: "push", Description: "Push the current branch to origin", Steps: []GitStep{{Args: []string{"push", "-u", "origin", "HEAD"}}}},
	{Name: "status", Description: "Show git status", Steps: []GitStep{{Args: []string{"status"}}}},
	{Name: "sync", Description: "Fetch, prune and pull from the remote", Steps: []GitStep{{Args: []string{"fetch", "-p"}, IgnoreFailure: true}, {Args: []string{"pull"}}}},
	{Name: "unwip", Description: "Undo the last commit and unstage it", Steps: []GitStep{{Args: []string{"reset", "--soft", "HEAD~1"}}, {Args: []string{"reset", "HEAD"}}}},
	{Name: "wip", Description: "Add all and commit as WIP", Steps: []GitStep{{Args: []string{"add", "--all"}}, {Args: []string{"commit", "-m", "WIP"}}}},
}

var DockerShortcuts = []DockerShortcut{
	{Name: "rm", Description: "Remove every container"},
	{Name: "rmi", Description: "Remove every image"},
	{Name: "reset", Description: "Remove every container, then every image"},
}

func commit(prefix string) []GitStep {
	return []GitStep{{Args: []string{"add", "."}}, {Args: []string{"commit", "-m"}, AppendArgs: prefix}}
}
