// Package mcp implements the Model Context Protocol server for kubectl-netdrill.
package mcp

import "time"

// Config holds MCP server defaults from CLI flags.
type Config struct {
	// Image is the netdrill container image used for create/debug tools.
	Image string
	// DefaultNamespace is used when a tool call omits namespace.
	DefaultNamespace string
	// Owner stamps and authorizes kubectl-netdrill.io/owner labels.
	Owner string
	// ExecTimeout bounds netdrill_pod_exec / netdrill_debug_exec calls.
	ExecTimeout time.Duration
	// MaxOutputBytes truncates captured exec stdout/stderr.
	MaxOutputBytes int64
	// InsecureAllowAnyPod disables owner/ticket guardrails when true.
	InsecureAllowAnyPod bool
	// MirrorExecToLogs writes command argv and captured stdout/stderr into
	// container PID 1 stdout (kubectl logs) when true. Default off: that
	// disclosure is broader than the MCP session (anyone with pods/log).
	MirrorExecToLogs bool
}
