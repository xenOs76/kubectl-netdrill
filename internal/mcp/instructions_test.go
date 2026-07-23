package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAgentInstructions(t *testing.T) {
	t.Parallel()

	assert.Contains(t, AgentInstructions, "https-wrench")
	assert.Contains(t, AgentInstructions, "nodeSelector")
	assert.Contains(t, AgentInstructions, "serviceAccount")
	assert.Contains(t, AgentInstructions, "netdrill://container-tools")
	assert.Contains(t, AgentInstructions, "kubernetes.io/hostname")
}
