// Package main is the entry point for kubectl-netdrill.
package main

import (
	"os"

	"github.com/xenos76/kubectl-netdrill/internal/cmd"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	return cmd.Execute()
}
