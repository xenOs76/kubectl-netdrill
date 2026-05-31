package mcp

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Deps bundles dependencies for MCP tool handlers.
type Deps struct {
	Client kubernetes.Interface
	Config *rest.Config
	Cfg    Config
	Guard  Guard
}
