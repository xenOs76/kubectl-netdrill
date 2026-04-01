# kubectl-netdrill

`kubectl-netdrill` is a plugin for `kubectl` and `krew` written in Go.

It is inspired by the [kubectl-netshoot](https://github.com/nilic/kubectl-netshoot) plugin.  

Similarly to `kubectl-netshoot`, it allows running a container in a pod and executing commands in it, running ephemeral containers and running network tools in the cluster.

It relies on a Docker image called `netdrill`, instead of `netshoot`.

For the development phase, the image is available at `ghcr.io/xenos76/netdrill:latest`.

## Current Status (v0.1.1)

The plugin is implemented and functional, with enhanced interactive terminal support.

### Implemented Features

- **`run` command**: Creates a temporary Pod and attaches to it.
- **`debug` command**: Adds an ephemeral container to an existing Pod.
- **`--host-network`**: Support for sharing the host's network namespace.
- **Interactive TTY**: Implements raw mode management and `TerminalSizeQueue` for immediate shell responsiveness and resizing support.
- **Sidecar-aware**: Correctly attaches to the `netdrill` container even in pods with multiple containers (e.g., Istio).
- **Build system**: Includes a `./build.sh` script and `./dist` directory management.

### Project Structure

- `main.go`: Entry point.
- `cmd/`: CLI commands (Cobra implementation).
- `pkg/k8s/`: Core logic for Kubernetes API interactions (Pod creation, attachment, ephemeral containers).
- `pkg/term/`: Terminal handling (size queue and raw mode).
- `deploy/`: Manifests for distribution (Krew).
- `dist/`: Build artifacts (ignored by git).

## Technical Architecture

### Kubernetes Interaction (`pkg/k8s`)

The plugin leverages the `k8s.io/client-go` library for direct communication with the Kubernetes API.

- **Pod Creation**: Standard `CoreV1().Pods().Create()` with a pre-configured `netdrill` container.
- **Ephemeral Containers**: Utilizes the `SubResource("ephemeralcontainers")` for the `debug` command, allowing troubleshooting without restarting the target Pod.
- **Attach/Exec**: Streams `stdin`, `stdout`, and `stderr` using `SPDYExecutor` to provide a real-time interactive shell.

### Terminal Management (`pkg/term`)

A key differentiator is the robust TTY support:

- **Raw Mode**: Temporarily puts the user's terminal into raw mode to pass special characters (like `Ctrl+C`) directly to the container.
- **Dynamic Resizing**: A `TerminalSizeQueue` monitors `SIGWINCH` signals to ensure the remote shell resizes correctly when the user's window changes.

## Development Workflow

### Building

Use the provided script to build for the current architecture:

```bash
./build.sh
```

### Linting

The project uses `golangci-lint`. Ensure all code passes before committing:

```bash
golangci-lint run
```

### Distribution

Releases are managed via `GoReleaser`. Configuration is in `.goreleaser.yaml`.

## Agents

### Operating Guidelines

As an agent, you represent the "brain" behind this plugin. When developing:

1. **Prioritize TTY**: Always ensure that shell interactions remain responsive and that terminal resizing works correctly.
2. **Minimize Footprint**: Favor standard Kubernetes libraries over heavy external dependencies.
3. **Keep Docs Sync**: Every feature update should be reflected in both `README.md` and this document.
4. **Sidecar Awareness**: When implementing features that search for containers, always default to or prioritize the `netdrill` image/container.

### Roadmap

- [ ] **Multi-architecture support**: Enhance `build.sh` and `GoReleaser` for ARM64.
- [ ] **Automatic Clean-up**: Ensure temporary pods are deleted even if the terminal session is forcefully closed.
- [ ] **Custom Toolkit Selection**: Allow users to specify a list of tools to be pre-installed if using a custom image.
