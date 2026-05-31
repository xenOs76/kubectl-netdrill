// Package mcp implements the Model Context Protocol server for kubectl-netdrill.
package mcp

import "time"

// Config holds MCP server defaults from CLI flags.
type Config struct {
	Image               string
	DefaultNamespace    string
	Owner               string
	ExecTimeout         time.Duration
	MaxOutputBytes      int64
	InsecureAllowAnyPod bool
}
