package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Run starts the MCP server on stdio until the client disconnects or ctx is canceled.
func Run(ctx context.Context, version string, deps *Deps) error {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "kubectl-netdrill",
		Version: version,
		Title:   "kubectl-netdrill MCP",
	}, nil)

	registerTools(server, deps)

	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		return fmt.Errorf("mcp server: %w", err)
	}

	return nil
}
