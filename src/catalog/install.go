package catalog

import (
	"strconv"
	"strings"
)

type Method string

const (
	MethodBrew   Method = "brew"
	MethodCask   Method = "cask"
	MethodMas    Method = "mas"
	MethodNpm    Method = "npm"
	MethodUv     Method = "uv"
	MethodBun    Method = "bun"
	MethodMise   Method = "mise"
	MethodScript Method = "script"
	MethodShell  Method = "shell"
	MethodApp    Method = "app"
	MethodSkill  Method = "skill"
)

type Install interface {
	Method() Method
	Key() string
	isInstall()
}

type BrewInstall struct {
	Formula string
	Trusted bool
}

type CaskInstall struct {
	Token   string
	Trusted bool
}

type MasInstall struct {
	Name string
	ID   int
}

type NpmInstall struct{ Package string }

type UvInstall struct{ Package string }

type BunInstall struct{ Package, Name string }

type MiseInstall struct{ Tool, Version string }

type ScriptInstall struct {
	URL  string
	Args []string
	Env  []string
}

type ShellInstall struct{ Line string }

type AppInstall struct{ Hint string }

type SkillInstall struct{ Repo, Skill string }

func (BrewInstall) Method() Method   { return MethodBrew }
func (CaskInstall) Method() Method   { return MethodCask }
func (MasInstall) Method() Method    { return MethodMas }
func (NpmInstall) Method() Method    { return MethodNpm }
func (UvInstall) Method() Method     { return MethodUv }
func (BunInstall) Method() Method    { return MethodBun }
func (MiseInstall) Method() Method   { return MethodMise }
func (ScriptInstall) Method() Method { return MethodScript }
func (ShellInstall) Method() Method  { return MethodShell }
func (AppInstall) Method() Method    { return MethodApp }
func (SkillInstall) Method() Method  { return MethodSkill }

func (i BrewInstall) Key() string { return lastSegment(i.Formula) }
func (i CaskInstall) Key() string { return lastSegment(i.Token) }
func (i MasInstall) Key() string  { return strconv.Itoa(i.ID) }
func (i NpmInstall) Key() string  { return i.Package }

func (i UvInstall) Key() string {
	name, _, _ := strings.Cut(i.Package, "[")
	return name
}

func (i BunInstall) Key() string {
	if i.Name != "" {
		return i.Name
	}
	return i.Package
}

func (i MiseInstall) Key() string   { return i.Tool }
func (i ScriptInstall) Key() string { return i.URL }
func (i ShellInstall) Key() string  { return i.Line }
func (i AppInstall) Key() string    { return i.Hint }
func (i SkillInstall) Key() string  { return i.Skill }

func (BrewInstall) isInstall()   {}
func (CaskInstall) isInstall()   {}
func (MasInstall) isInstall()    {}
func (NpmInstall) isInstall()    {}
func (UvInstall) isInstall()     {}
func (BunInstall) isInstall()    {}
func (MiseInstall) isInstall()   {}
func (ScriptInstall) isInstall() {}
func (ShellInstall) isInstall()  {}
func (AppInstall) isInstall()    {}
func (SkillInstall) isInstall()  {}

func Brew(formula string) Install        { return BrewInstall{Formula: formula} }
func TrustedBrew(formula string) Install { return BrewInstall{Formula: formula, Trusted: true} }
func Cask(token string) Install          { return CaskInstall{Token: token} }
func Mas(name string, id int) Install    { return MasInstall{Name: name, ID: id} }
func Npm(pkg string) Install             { return NpmInstall{Package: pkg} }
func Uv(pkg string) Install              { return UvInstall{Package: pkg} }
func Bun(pkg string) Install             { return BunInstall{Package: pkg} }
func BunAs(source, name string) Install  { return BunInstall{Package: source, Name: name} }
func Mise(tool, version string) Install  { return MiseInstall{Tool: tool, Version: version} }
func Shell(line string) Install          { return ShellInstall{Line: line} }
func App(hint string) Install            { return AppInstall{Hint: hint} }
func Skill(repo, skill string) Install   { return SkillInstall{Repo: repo, Skill: skill} }

func Script(url string, args ...string) Install { return ScriptInstall{URL: url, Args: args} }

func ScriptEnv(url string, env []string, args ...string) Install {
	return ScriptInstall{URL: url, Args: args, Env: env}
}

func lastSegment(name string) string {
	return name[strings.LastIndex(name, "/")+1:]
}
