package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDebugCommandUse(t *testing.T) {
	assert.Equal(t, "debug [POD]", debugCmd.Use)
}

func TestDebugCommandShort(t *testing.T) {
	assert.NotEmpty(t, debugCmd.Short)
}

func TestDebugCommandLong(t *testing.T) {
	assert.NotEmpty(t, debugCmd.Long)
}

func TestDebugCommandArgs(t *testing.T) {
	assert.NotNil(t, debugCmd.Args)
}

func TestDebugFlags(t *testing.T) {
	flag := debugCmd.Flags().Lookup("target")
	assert.NotNil(t, flag, "flag target should exist")
}
