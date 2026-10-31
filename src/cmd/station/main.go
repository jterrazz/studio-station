package main

import (
	"os"

	"github.com/jterrazz/studio-station/src/app"
)

func main() {
	os.Exit(app.Run(os.Args[1:], os.Environ(), os.Stdin, os.Stdout, os.Stderr))
}
