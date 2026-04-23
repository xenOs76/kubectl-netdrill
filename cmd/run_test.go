package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunCommandUse(t *testing.T) {
	assert.Equal(t, "run [NAME]", runCmd.Use)
}

func TestRunCommandShort(t *testing.T) {
	assert.NotEmpty(t, runCmd.Short)
}

func TestRunCommandLong(t *testing.T) {
	assert.NotEmpty(t, runCmd.Long)
}

func TestRunFlags(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
	}{
		{"host-network flag exists", "host-network"},
		{"command flag exists", "command"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if runCmd.Parent() == nil {
				rootCmd.AddCommand(runCmd)
			}

			flag := runCmd.Flag(tt.flagName)
			assert.NotNil(t, flag, "flag %s should exist", tt.flagName)
		})
	}
}
