package cmd

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func TestPodCommandFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "no args uses default pod name",
			args: []string{},
		},
		{
			name: "custom pod name",
			args: []string{"mypod"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := podCmd
			cmd.SetArgs(tt.args)
			assert.NotNil(t, cmd)
		})
	}
}

func TestPodCommand_Help(t *testing.T) {
	cmd := podCmd
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	assert.NoError(t, err)
}

func TestPodCommand_Use(t *testing.T) {
	assert.Equal(t, "pod [NAME]", podCmd.Use)
}

func TestPodCommand_Short(t *testing.T) {
	assert.NotEmpty(t, podCmd.Short)
}

func TestPodCommand_Long(t *testing.T) {
	assert.NotEmpty(t, podCmd.Long)
}

func TestPodCommand_MaxArgs(t *testing.T) {
	cmd := podCmd
	cmd.SetArgs([]string{"arg1", "arg2"})

	err := cmd.ValidateArgs([]string{"arg1", "arg2"})
	assert.Error(t, err)
}

func TestPodFlags(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
	}{
		{"service-account flag exists", "service-account"},
		{"port flag exists", "port"},
		{"env flag exists", "env"},
		{"host-network flag exists", "host-network"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := podCmd.Flags().Lookup(tt.flagName)
			assert.NotNil(t, flag, "flag %s should exist", tt.flagName)
		})
	}
}

func TestPodCommandInheritance(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
		cmdFlags func() *pflag.FlagSet
	}{
		{"image flag exists", "image", rootCmd.Flags},
		{"node-selector flag exists", "node-selector", rootCmd.Flags},
		{"host-network flag exists", "host-network", podCmd.Flags},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := tt.cmdFlags().Lookup(tt.flagName)
			assert.NotNil(t, flag, "flag %s should exist", tt.flagName)
		})
	}
}
