package main

import (
	"fmt"
	"os"

	"github.com/uta8a/github-policy-cli/internal/cli"
)

// version is injected from the validated release tag with -ldflags.
var version = "dev"

func main() {
	command := cli.New(nil)
	command.Version = version
	if err := command.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
