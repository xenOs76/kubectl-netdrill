package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/xenos76/kubectl-netdrill/internal/k8s"
)

const containerLogMirrorTimeout = 10 * time.Second

// formatContainerExecLog builds the audit text mirrored into container logs.
func formatContainerExecLog(command []string, stdout, stderr string, exitCode int) string {
	cmdJSON, err := json.Marshal(command)
	if err != nil {
		cmdJSON = []byte(`["<unserializable>"]`)
	}

	var b strings.Builder

	b.WriteString("--- netdrill-mcp exec ---\n")
	fmt.Fprintf(&b, "command: %s\n", cmdJSON)
	fmt.Fprintf(&b, "exit: %d\n", exitCode)

	if stdout != "" {
		b.WriteString("--- stdout ---\n")
		b.WriteString(stdout)

		if !strings.HasSuffix(stdout, "\n") {
			b.WriteByte('\n')
		}
	}

	if stderr != "" {
		b.WriteString("--- stderr ---\n")
		b.WriteString(stderr)

		if !strings.HasSuffix(stderr, "\n") {
			b.WriteByte('\n')
		}
	}

	b.WriteString("--- end ---\n")

	return b.String()
}

// mirrorExecToContainerLog best-effort writes the exec audit trail to the
// container's PID 1 stdout so it appears in kubectl logs. Failures are logged
// on the MCP host and never change the agent-facing exec result.
func mirrorExecToContainerLog(
	ctx context.Context,
	deps *Deps,
	ns, podName, container string,
	command []string,
	result k8s.ExecResult,
) {
	text := formatContainerExecLog(command, result.Stdout, result.Stderr, result.ExitCode)
	text, _ = deps.Guard.Truncate(text)
	encoded := base64.StdEncoding.EncodeToString([]byte(text))

	mirrorCtx, cancel := context.WithTimeout(ctx, containerLogMirrorTimeout)
	defer cancel()

	mirrorCmd := []string{
		"sh", "-c", `printf '%s\n' "$1" | base64 -d > /proc/1/fd/1`,
		"_", encoded,
	}

	if _, err := k8s.ExecCommand(
		mirrorCtx, deps.Client, deps.Config, ns, podName, container,
		mirrorCmd, deps.Guard.MaxOutput,
	); err != nil {
		slog.Warn("failed to mirror exec to container logs",
			"namespace", ns, "pod", podName, "container", container, "err", err)
	}
}
