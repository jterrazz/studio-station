package catalog

type DockerShortcut struct{ Name, Description, Verb string }

var DockerShortcuts = []DockerShortcut{
	{Name: "rm", Description: "Remove every container", Verb: "remove every container"},
	{Name: "rmi", Description: "Remove every image", Verb: "remove every image"},
	{Name: "reset", Description: "Remove every container, then every image", Verb: "remove every container and image"},
}
