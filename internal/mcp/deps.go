package mcp

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Deps bundles dependencies for MCP tool handlers.
type Deps struct {
	// Client is the Kubernetes API client used by tool handlers.
	Client kubernetes.Interface
	// Config is the REST config for attach/exec streaming.
	Config *rest.Config
	// Cfg holds MCP server defaults from CLI flags.
	Cfg Config
	// Guard enforces owner/ticket authorization on managed resources.
	Guard Guard
}
