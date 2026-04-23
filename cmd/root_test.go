package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCmdExecute(t *testing.T) {
	rootCmd.SetArgs([]string{"--help"})
	err := rootCmd.Execute()
	require.NoError(t, err)
}

func TestRootCmdVersion(t *testing.T) {
	rootCmd.SetArgs([]string{"--version"})
	err := rootCmd.Execute()
	require.NoError(t, err)
}

func TestRootFlags(t *testing.T) {
	flags := rootCmd.Flags()

	tests := []struct {
		name     string
		flagName string
	}{
		{"image flag exists", "image"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := flags.Lookup(tt.flagName)
			assert.NotNil(t, flag)
		})
	}
}

func TestConfigFlags(t *testing.T) {
	assert.NotNil(t, ConfigFlags)
}

func TestDefaultImage(t *testing.T) {
	assert.Equal(t, "ghcr.io/xenos76/netdrill:latest", DefaultImage)
}

func TestVersion(t *testing.T) {
	assert.Equal(t, "dev", Version)
}
