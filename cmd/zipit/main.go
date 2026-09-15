package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/poizdev/zipit/internal/cli"
)

func main() {
	if err := cli.NewRootCommand(os.Stdout, os.Stderr).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(exitCode(err))
	}
}

func exitCode(err error) int {
	if errors.Is(err, cli.ErrInterrupted) {
		return 130
	}
	return 1
}
