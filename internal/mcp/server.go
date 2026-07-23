package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Run starts the MCP server on stdio until the client disconnects or ctx is canceled.
func Run(ctx context.Context, version string, deps *Deps) error {
	return RunTransport(ctx, version, deps, &mcp.StdioTransport{})
}

// RunTransport starts the MCP server on the given transport until disconnect or cancel.
func RunTransport(ctx context.Context, version string, deps *Deps, transport mcp.Transport) error {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "kubectl-netdrill",
		Version: version,
		Title:   "kubectl-netdrill MCP",
	}, &mcp.ServerOptions{
		Instructions: AgentInstructions,
	})

	registerTools(server, deps)

	if err := registerContainerTools(server, deps); err != nil {
		return fmt.Errorf("register container tools: %w", err)
	}

	if err := server.Run(ctx, transport); err != nil {
		return fmt.Errorf("mcp server: %w", err)
	}

	return nil
}
