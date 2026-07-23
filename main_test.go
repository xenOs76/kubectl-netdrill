package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRun_Help(t *testing.T) {
	orig := os.Args

	defer func() { os.Args = orig }()

	os.Args = []string{"kubectl-netdrill", "--help"}

	require.NoError(t, run())
}
