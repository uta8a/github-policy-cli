package main

import (
	"fmt"
	"os"

	"github.com/uta8a/github-policy-cli/internal/cli"
)

func main() {
	if err := cli.New(nil).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
