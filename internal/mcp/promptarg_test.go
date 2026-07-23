package mcp

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
)

func TestPromptArg(t *testing.T) {
	t.Parallel()

	assert.Empty(t, promptArg(nil, "target"))
	assert.Empty(t, promptArg(&mcp.GetPromptRequest{}, "target"))
	assert.Empty(t, promptArg(&mcp.GetPromptRequest{
		Params: &mcp.GetPromptParams{},
	}, "target"))
	assert.Empty(t, promptArg(&mcp.GetPromptRequest{
		Params: &mcp.GetPromptParams{Arguments: map[string]string{}},
	}, "target"))
	assert.Equal(t, "8.8.8.8", promptArg(&mcp.GetPromptRequest{
		Params: &mcp.GetPromptParams{Arguments: map[string]string{"target": "8.8.8.8"}},
	}, "target"))
}
