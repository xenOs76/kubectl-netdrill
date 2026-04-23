package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeploymentCommandFlags(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
	}{
		{"replicas flag exists", "replicas"},
		{"cpu-request flag exists", "cpu-request"},
		{"memory-request flag exists", "memory-request"},
		{"cpu-limit flag exists", "cpu-limit"},
		{"memory-limit flag exists", "memory-limit"},
		{"labels flag exists", "labels"},
		// Inherited flags
		{"service-account flag exists", "service-account"},
		{"port flag exists", "port"},
		{"env flag exists", "env"},
		{"host-network flag exists", "host-network"},
		{"node-selector flag exists", "node-selector"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure parent is set so persistent flags are visible
			if deploymentCmd.Parent() == nil {
				rootCmd.AddCommand(deploymentCmd)
			}

			flag := deploymentCmd.Flag(tt.flagName)
			assert.NotNil(t, flag, "flag %s should exist", tt.flagName)
		})
	}
}

func TestDeploymentCommand_Use(t *testing.T) {
	assert.Equal(t, "deployment [NAME]", deploymentCmd.Use)
}
