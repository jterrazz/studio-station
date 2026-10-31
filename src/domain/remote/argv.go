package remote

import "strings"

const script = `PATH="$HOME/.local/bin:/usr/local/bin:$PATH" exec station "$@"`

func Argv(machine string, interactive bool, words []string) []string {
	argv := []string{}
	if interactive {
		argv = append(argv, "-t")
	}
	argv = append(argv, machine, "--", "sh", "-c", Quote(script), "station")
	for _, w := range words {
		argv = append(argv, Quote(w))
	}
	return argv
}

func Quote(word string) string {
	if word != "" && strings.IndexFunc(word, unsafe) < 0 {
		return word
	}
	return "'" + strings.ReplaceAll(word, "'", `'\''`) + "'"
}

func unsafe(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return false
	}
	return !strings.ContainsRune("_@%+=:,./-", r)
}

func Words(args []string) []string {
	words := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--":
			return append(words, args[i:]...)
		case a == "--on":
			i++
		case strings.HasPrefix(a, "--on="):
		default:
			words = append(words, a)
		}
	}
	return words
}
