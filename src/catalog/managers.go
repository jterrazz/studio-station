package catalog

type Manager string

const (
	ManagerBrew   Manager = "brew"
	ManagerNpm    Manager = "npm"
	ManagerBun    Manager = "bun"
	ManagerUv     Manager = "uv"
	ManagerMise   Manager = "mise"
	ManagerSkills Manager = "skills"
)

type ManagerMethods struct {
	Manager Manager
	Methods []Method
}

var Managers = []ManagerMethods{
	{ManagerBrew, []Method{MethodBrew, MethodCask}},
	{ManagerNpm, []Method{MethodNpm}},
	{ManagerBun, []Method{MethodBun}},
	{ManagerUv, []Method{MethodUv}},
	{ManagerMise, []Method{MethodMise}},
	{ManagerSkills, []Method{MethodSkill}},
}
