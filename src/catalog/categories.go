package catalog

type Category string

const (
	CategoryPackageManagers Category = "Package Managers"
	CategoryRuntimes        Category = "Runtimes"
	CategoryShellTerminal   Category = "Shell & Terminal"
	CategoryCLIApps         Category = "Command line"
	CategoryGit             Category = "Git"
	CategoryEditors         Category = "Editors & IDEs"
	CategoryContainersVMs   Category = "Containers & VMs"
	CategoryDeploy          Category = "Deploy"
	CategoryAIAgents        Category = "AI Agents"
	CategoryAITooling       Category = "AI Tooling"
	CategoryAIApps          Category = "AI Apps"
	CategoryBrowsers        Category = "Browsers"
	CategoryCommunication   Category = "Communication"
	CategoryProductivity    Category = "Productivity"
	CategoryMedia           Category = "Media"
	CategoryRemoteAccess    Category = "Remote Access"
	CategorySecurityApps    Category = "Security"
	CategorySystemUtilities Category = "System Utilities"
	CategoryAgentSkills     Category = "Agent skills"
)

var Categories = []Category{
	CategoryPackageManagers,
	CategoryRuntimes,
	CategoryShellTerminal,
	CategoryCLIApps,
	CategoryGit,
	CategoryEditors,
	CategoryContainersVMs,
	CategoryDeploy,
	CategoryAIAgents,
	CategoryAITooling,
	CategoryAIApps,
	CategoryBrowsers,
	CategoryCommunication,
	CategoryProductivity,
	CategoryMedia,
	CategoryRemoteAccess,
	CategorySecurityApps,
	CategorySystemUtilities,
	CategoryAgentSkills,
}
