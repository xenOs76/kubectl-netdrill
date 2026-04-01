# kubectl-netdrill

`kubectl-netdrill` is a `kubectl` plugin designed for network troubleshooting
within Kubernetes clusters. It uses the custom
[netdrill](https://github.com/xenOs76/netdrill) container image for diagnostic
tools.

## Acknowledgements

`kubeclt-netdrill` is heavily inspired by
[netshoot](https://github.com/nicolaka/netshoot) and
[kubectl-netshoot](https://github.com/nilic/kubectl-netshoot).

## Features

- **`run`**: Quickly spin up a temporary, interactive troubleshooting Pod.
- **`debug`**: Inject an ephemeral container into an existing Pod to
  troubleshoot within its network context.

## Installation

### Prerequisites

- [Go 1.25+](https://go.dev/)
- `kubectl` configured with cluster access

### Building from Source

1. Clone the repository:

   ```bash
   git clone https://github.com/xenos76/kubectl-netdrill
   cd kubectl-netdrill
   ```

2. Build using the provided script:

   ```bash
   ./build.sh
   ```

3. The binary will be available at `dist/kubectl-netdrill`. You can move it to
   your `PATH` or use it directly:

   ```bash
   sudo mv dist/kubectl-netdrill /usr/local/bin/
   ```

## Usage

### Running a Temporary Pod

Create a throw-away pod for general network testing:

```bash
kubectl netdrill run my-troubleshooter
```

### Debugging an Existing Pod

Inject a `netdrill` container into a running Pod:

```bash
kubectl netdrill debug <pod-name>
```

To target a specific container's PID namespace:

```bash
kubectl netdrill debug <pod-name> --target <container-name>
```

### Node Troubleshooting (Host Network)

Run a pod that shares the host's network namespace:

```bash
kubectl netdrill run node-diag --host-network
```

## Configuration

Standard `kubectl` flags are supported:

- `-n`, `--namespace`: Target namespace.
- `--context`: Kubeconfig context.
- `-i`, `--image`: Override the default `netdrill` image.

## Documentation

For more detailed technical information, architecture diagrams, and guidelines
for AI agents or developers, see [docs/AGENTS.md](docs/AGENTS.md).
