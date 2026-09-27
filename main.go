// Package main is the entry point for kubectl-netdrill.
package main

import (
	"os"

	"github.com/xenos76/kubectl-netdrill/internal/cmd"
)

// main runs the kubectl-netdrill application and exits with non-zero code on error.
func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

// run executes the root command and returns any execution error.
func run() error {
	return cmd.Execute()
}
