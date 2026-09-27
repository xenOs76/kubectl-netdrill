# kubectl-netdrill

`kubectl-netdrill` is a plugin for `kubectl` and `krew` written in Go.

It is inspired by the
[kubectl-netshoot](https://github.com/nilic/kubectl-netshoot) plugin.

Similarly to `kubectl-netshoot`, it allows running a container in a pod and
executing commands in it, running ephemeral containers and running network tools
in the cluster.

It relies on a Docker image called `netdrill`, instead of `netshoot`.

The image is available at `ghcr.io/xenos76/netdrill:latest`.

The program uses Cobra for the CLI. Packages live under `internal/` (not a
public library API). Kubernetes access uses `client-go`, following
[Krew plugin guidelines](https://krew.sigs.k8s.io/docs/). Interactive attach uses
`internal/term` for raw mode and resize. Code is validated with `golangci-lint`
and Go tests; `./build.sh` produces the binary.

Agents must activate all the skills present in the folder `.agents` at the top
level of current repository.

## Current Status (v0.1.2+)

The plugin is implemented and functional, with interactive TTY support and an
MCP server for AI agents.

### Implemented Features

- **`pod` command**: Persistent Pod (`--service-account`, `--env`, `--port`,
  `--host-network`, `--node-selector`, `--labels`).
- **`deployment` command**: Deployment with replicas, resources, labels, and the
  same pod options.
- **`debug` command**: Ephemeral `netdrill-debug` container (`--target` for PID
  namespace sharing).
- **`mcp` command**: MCP stdio server (pod/deployment lifecycle, non-interactive
  exec, owner/ticket guardrails, image resolve, agent instructions).
- **`completion` command**: bash/zsh/fish/powershell completion scripts.
- **Interactive TTY**: Raw mode + `TerminalSizeQueue` for attach/resize.
- **Sidecar-aware**: Prefers the `netdrill` container when attaching.
- **Build/release**: `./build.sh`, GoReleaser (completions + man pages).

### Project Structure

- `main.go`: Entry point.
- `internal/cmd/`: CLI commands (Cobra).
- `internal/k8s/`: Kubernetes API (pods, deployments, attach, exec, IRSA token).
- `internal/mcp/`: MCP server, tools, guard, prompts, agent instructions.
- `internal/image/`: GHCR `:latest` → highest-semver resolve (MCP).
- `internal/netdrill/`: Shared labels and config types.
- `internal/term/`: Terminal handling (size queue and raw mode).
- `dist/`: Build artifacts (ignored by git).

## Technical Architecture

### MCP server (`internal/mcp`)

- Transport: `mcp.StdioTransport` only in v1.
- Logging: `log/slog` to stderr; never write to stdout on the MCP path.
- `--owner` (default `$USER`) stamps `kubectl-netdrill.io/owner` on creates;
  delete/exec authorize against live pod labels (not pod name alone).
- Optional `ticketId` on tools sets/requires `kubectl-netdrill.io/ticket`.
- **Server instructions** (`AgentInstructions`): prefer image CLIs
  (`https-wrench`, `aws-probe`, `doggo`); pass `nodeSelector` /
  `serviceAccount` / `env` / `ports` / `hostNetwork` / `labels` on create tools.
- **Create parity with CLI:** create tools accept `nodeSelector`,
  `serviceAccount`, `env`, `ports`, `hostNetwork`, `labels`; deployment also accepts
  `replicas`, `cpuRequest`, `memoryRequest`, `cpuLimit`,
  `memoryLimit`. Interactive TTY attach remains CLI-only.
- **Container tools (always on):** resource `netdrill://container-tools` and
  prompts describe CLIs in the netdrill image. Agents run them via
  `netdrill_pod_exec` (IRSA via `serviceAccount`). Do not register
  `aws_probe_*` tools here—use standalone `aws-probe mcp` for host AWS access.
- **Image pin (MCP):** when `--image` ends with `:latest` (default), `mcp`
  resolves the highest semver tag from GHCR once at startup (`internal/image`)
  and uses that for create/debug tools; registry errors log a warning and fall
  back. Explicit `-i repo:tag` or `--resolve-image=false` skips lookup.

### Kubernetes Interaction (`internal/k8s`)

The plugin uses `k8s.io/client-go` for the Kubernetes API.

- **Pod Creation**: `CoreV1().Pods().Create()` with a pre-configured `netdrill`
  container (optional EKS IRSA token projection when using a ServiceAccount).
- **Ephemeral Containers**: `SubResource("ephemeralcontainers")` for `debug`.
- **Attach/Exec**: SPDY streams for interactive attach (CLI) and non-interactive
  exec (MCP). With `--mirror-exec-to-logs` (default off), MCP exec best-effort
  mirrors command+output to the container's PID 1 stdout (`kubectl logs`); that
  disclosure is visible to anyone with `pods/log` access.

### Terminal Management (`internal/term`)

- **Raw Mode**: Passes special characters (for example `Ctrl+C`) to the container.
- **Dynamic Resizing**: `TerminalSizeQueue` on `SIGWINCH` for remote shell resize.

## Development Workflow

### Building

```bash
./build.sh
```

### Linting

```bash
golangci-lint run
```

### Distribution

Releases are managed via `GoReleaser`. Configuration is in `.goreleaser.yaml`.

## Agents

### Operating Guidelines

1. **Prioritize TTY**: Keep shell attach responsive and resizing correct.
2. **Minimize Footprint**: Prefer standard Kubernetes libraries.
3. **Keep Docs Sync**: Reflect feature updates in both `README.md` and this
   document.
4. **Sidecar Awareness**: Prefer the `netdrill` image/container when selecting
   containers.
5. **MCP parity**: When adding CLI create flags, expose matching MCP tool params
   and update `AgentInstructions` / `container-tools.md` / README.

### Roadmap

- [x] **Multi-architecture support**: Enhance `build.sh` and `GoReleaser` for
      ARM64.
- [x] **Automatic Clean-up**: Ensure temporary pods are deleted even if the
      terminal session is forcefully closed.
- [x] **Add NodeSelector option**: Allow to select the K8s node where the Pod
      will be created using K8s Node labels.
- [x] **deployment command**: Implement Deployment support as a superset of the
      pod command.
- [x] **MCP create parity**: Expose serviceAccount/env/ports/hostNetwork and
      deployment resources on MCP tools.
- [ ] **Iperf3 listening port**: Iperf3 in server mode needs an open port to
      receive traffic from clients
