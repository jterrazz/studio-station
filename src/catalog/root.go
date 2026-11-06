package catalog

const Sudo = "sudo" // value: the one spelling of sudo outside the owner's words: station runs a line as root only inside a VM of its own, or names one for the owner to type.

func AsRoot(line string) string { return Sudo + " " + line }
