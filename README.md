# kubectl-netdrill

<p align="center">
    <img width="450" alt="NetDrill Logo" src="./assets/img/netdrill-logo.png"/><br />
    <i>To find a way. Any way.</i>
</p>

`kubectl-netdrill` is a `kubectl` plugin designed for network troubleshooting
within Kubernetes clusters.\
It relies on the diagnostic tools provided by the
[netdrill](https://github.com/xenOs76/netdrill) container image.

## Acknowledgements

`kubeclt-netdrill` is heavily inspired by
[netshoot](https://github.com/nicolaka/netshoot) and
[kubectl-netshoot](https://github.com/nilic/kubectl-netshoot).

## Installation

<details>

### Go install

```shell
go install github.com/xenos76/kubectl-netdrill@latest
```

### Manual download

Release binaries and DEB, RPM, APK packages can be downloaded from the
[repo's releases section](https://github.com/xenOs76/kubectl-netdrill/releases).\
Binaries and packages are built for Linux and MacOS, `amd64` and `arm64`.

### APT

Configure the repo the following way:

```shell
echo "deb [trusted=yes] https://repo.os76.xyz/apt stable main" | sudo tee /etc/apt/sources.list.d/os76.list
```

then:

```shell
sudo apt-get update && sudo apt-get install -y kubectl-netdrill
```

### YUM

Configure the repo the following way:

```shell
echo '[os76]
name=OS76 Yum Repo
baseurl=https://repo.os76.xyz/yum/$basearch/
enabled=1
gpgcheck=0
repo_gpgcheck=0' | sudo tee /etc/yum.repos.d/os76.repo
```

then:

```shell
sudo yum install kubectl-netdrill
```

### Homebrew

Add Os76 Homebrew repository:

```shell
brew tap xenos76/tap
```

Install `kubectl-netdrill`:

```shell
brew install --casks kubectl-netdrill
```

Note: `kubectl-netdrill` is not configured and signed as a MacOS app. Manual
steps might be needed to enable the execution of the binary.

### Krew

```shell
❯ krew index add os76 https://github.com/xenOs76/krews.git
WARNING: You have added a new index from "https://github.com/xenOs76/krews.git"
The plugins in this index are not audited for security by the Krew maintainers.
Install them at your own risk.

❯ krew index list
INDEX     URL
default   https://github.com/kubernetes-sigs/krew-index.git
netshoot  https://github.com/nilic/kubectl-netshoot.git
os76      https://github.com/xenOs76/krews.git

❯ krew search netdrill
NAME           DESCRIPTION                                         INSTALLED
os76/netdrill  kubectl-netdrill, a network troubleshooting plu...  no

❯ krew install os76/netdrill
Updated the local copy of plugin index.
Updated the local copy of plugin index "netshoot".
Updated the local copy of plugin index "os76".
Installing plugin: netdrill
Installed plugin: netdrill
\
 | Use this plugin:
 |      kubectl netdrill
 | Documentation:
 |      https://github.com/xenOs76/kubectl-netdrill
/

❯ ~/.krew/bin/kubectl-netdrill -v
kubectl-netdrill version 0.1.2
```

</details>

## Build

<details>

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

</details>

## Usage

### Quick Reference

| Command                              | Description                                                   |
| ------------------------------------ | ------------------------------------------------------------- |
| `kubectl netdrill run [name]`        | Create a temporary troubleshooting pod (auto-deleted on exit) |
| `kubectl netdrill pod [name]`        | Create a persistent troubleshooting pod                       |
| `kubectl netdrill deployment [name]` | Create a persistent troubleshooting deployment                |
| `kubectl netdrill debug <pod>`       | Inject ephemeral container into a running pod                 |
| `kubectl netdrill mcp`               | Run MCP server (stdio) for AI agent integration               |
| `kubectl netdrill completion <shell>` | Generate shell completion (bash/zsh/fish/powershell)         |

### kubectl netdrill run

<details>

Create a throw-away pod for general network testing:

```bash
kubectl netdrill run my-troubleshooter
```

#### Options

| Flag              | Short | Description                                                    | Default                           |
| ----------------- | ----- | -------------------------------------------------------------- | --------------------------------- |
| `--host-network`  |       | Use the host's network namespace                               | `false`                           |
| `--command`       |       | Command to run in the container                                | shell (interactive)               |
| `--node-selector` |       | Node labels for pod scheduling (e.g. `kubernetes.io/os=linux`) |                                   |
| `--image`         | `-i`  | Container image to use                                         | `ghcr.io/xenos76/netdrill:latest` |

#### Examples

Create a default troubleshooting pod:

```bash
kubectl netdrill run
```

Create a pod with a custom name:

```bash
kubectl netdrill run my-troubleshooter
```

Run a pod with host network access (for node-level troubleshooting):

```bash
kubectl netdrill run node-diag --host-network
```

Schedule a pod on a specific node:

```bash
kubectl netdrill run my-pod --node-selector kubernetes.io/os=linux
```

Run a custom command instead of an interactive shell:

```bash
kubectl netdrill run --command "ip addr show"
```

</details>

### kubectl netdrill pod

<details>

Create a persistent troubleshooting pod that remains running after you exit
(unlike `run` which auto-deletes):

```bash
kubectl netdrill pod my-persistent-troubleshooter
```

#### Options

| Flag                | Short | Description                                          | Default                           |
| ------------------- | ----- | ---------------------------------------------------- | --------------------------------- |
| `--service-account` |       | ServiceAccount to use for the pod                    |                                   |
| `--port`            |       | Ports to expose on the container (e.g., `--port 80`) |                                   |
| `--env`             |       | Environment variables (e.g., `--env KEY=VALUE`)      |                                   |
| `--host-network`    |       | Use the host's network namespace                     | `false`                           |
| `--node-selector`   |       | Node labels for pod scheduling                       |                                   |
| `--image`           | `-i`  | Container image to use                               | `ghcr.io/xenos76/netdrill:latest` |

#### Examples

Create a persistent troubleshooting pod:

```bash
kubectl netdrill pod
```

Create a pod with a custom service account:

```bash
kubectl netdrill pod --service-account node-troubleshooter
```

Create a pod and expose a port:

```bash
kubectl netdrill pod --port 80 --port 443
```

Create a pod with environment variables:

```bash
kubectl netdrill pod --env DEBUG=true --env API_URL=http://api:8080
```

Create a pod with a specific IAM role in EKS:

```bash
kubectl-netdrill pod netdrill-iam-pod \
    --env AWS_REGION='eu-central-1' \
    --env AWS_ROLE_ARN='arn:aws:iam::AWS_ACCOUNT_ID:role/IAMRoleName' \
    --env AWS_WEB_IDENTITY_TOKEN_FILE='/run/secrets/eks.amazonaws.com/serviceaccount/token' \
    --service-account aws-client
```

Attach to an existing persistent pod:

```bash
kubectl exec -it my-persistent-troubleshooter -n default -- /bin/bash
```

</details>

### kubectl netdrill deployment

<details>

Create a persistent troubleshooting deployment. This is useful when you need a
highly available or scalable troubleshooting environment:

```bash
kubectl netdrill deployment my-deployment
```

#### Options

| Flag                | Short | Description                                          | Default                           |
| ------------------- | ----- | ---------------------------------------------------- | --------------------------------- |
| `--replicas`        |       | Number of replicas                                   | `1`                               |
| `--cpu-request`     |       | CPU request (e.g. `100m`)                            |                                   |
| `--memory-request`  |       | Memory request (e.g. `128Mi`)                        |                                   |
| `--cpu-limit`       |       | CPU limit (e.g. `200m`)                              |                                   |
| `--memory-limit`    |       | Memory limit (e.g. `256Mi`)                          |                                   |
| `--labels`          |       | Additional labels (e.g. `key=val`)                   |                                   |
| `--service-account` |       | ServiceAccount to use for the deployment             |                                   |
| `--port`            |       | Ports to expose on the container (e.g., `--port 80`) |                                   |
| `--env`             |       | Environment variables (e.g., `--env KEY=VALUE`)      |                                   |
| `--host-network`    |       | Use the host's network namespace                     | `false`                           |
| `--node-selector`   |       | Node labels for pod scheduling                       |                                   |
| `--image`           | `-i`  | Container image to use                               | `ghcr.io/xenos76/netdrill:latest` |

#### Examples

Create a deployment with 3 replicas:

```bash
kubectl netdrill deployment my-deploy --replicas 3
```

Create a deployment with resource limits:

```bash
kubectl netdrill deployment my-deploy --cpu-limit 200m --memory-limit 256Mi
```

Create a deployment with custom labels:

```bash
kubectl netdrill deployment my-deploy --labels env=prod,team=network
```

</details>

### kubectl netdrill debug

<details>

Inject a `netdrill` container into a running Pod to troubleshoot within its
network context:

```bash
kubectl netdrill debug <pod-name>
```

To target a specific container's PID namespace (share process namespace):

```bash
kubectl netdrill debug <pod-name> --target <container-name>
```

#### Options

| Flag       | Short | Description                                | Default                           |
| ---------- | ----- | ------------------------------------------ | --------------------------------- |
| `--target` |       | Container name to share PID namespace with |                                   |
| `--image`  | `-i`  | Container image to use                     | `ghcr.io/xenos76/netdrill:latest` |

#### Examples

Debug a specific pod:

```bash
kubectl netdrill debug my-app-pod
```

Debug with process namespace sharing (to see same network interfaces as target
container):

```bash
kubectl netdrill debug my-app-pod --target my-app-container
```

</details>

### kubectl netdrill completion

<details>

Generate shell completion scripts:

```bash
source <(kubectl-netdrill completion bash)
kubectl-netdrill completion zsh > "${fpath[1]}/_kubectl-netdrill"
```

Supported shells: `bash`, `zsh`, `fish`, `powershell`.

</details>

### kubectl netdrill mcp

<details>

Start a [Model Context Protocol](https://modelcontextprotocol.io/) server on
stdin/stdout so AI clients (Cursor, Claude Desktop, etc.) can create netdrill
pods, run commands inside them, and clean up—without interactive TTY attach.

```bash
kubectl netdrill mcp --owner "$USER" -n my-namespace
```

The server always registers an MCP resource `netdrill://container-tools`
(catalog of CLIs in the [netdrill image](https://github.com/xenos76/netdrill),
including `aws-probe` and `https-wrench`) and prompts that guide agents to run
them via **`netdrill_pod_exec`** inside pods. It publishes **server
instructions** so agents prefer in-image tools (especially `https-wrench` for
TLS/HTTPS), pass **`nodeSelector`** when the user names a host, and set
**`serviceAccount`** for EKS IRSA. It does **not** register `aws_probe_*` API
tools; for direct AWS MCP from your workstation, use a separate
**`aws-probe mcp`** server.

When `--image` uses the `:latest` tag (the default), MCP startup queries GHCR
for the highest semver tag (for example `v0.1.2`) and pins that reference for
all create/debug tools in the session. If the registry is unreachable, a warning
is logged and the configured image is used. Pass an explicit tag with `-i` to
skip resolution, or `--resolve-image=false` for air-gapped use.

#### Cursor configuration example

```json
{
  "mcpServers": {
    "kubectl-netdrill": {
      "command": "kubectl-netdrill",
      "args": [
        "mcp",
        "--owner", "xeno",
        "-n", "troubleshooting",
        "--context", "my-cluster",
        "--resolve-image=true"
      ]
    }
  }
}
```

#### MCP settings matrix

| Setting | Flag / env | Purpose | Recommended |
| ------- | ---------- | ------- | ----------- |
| Owner | `--owner` / `$USER` | Label for create/delete/exec auth | Always set explicitly |
| Namespace | `-n` / `--namespace` | Default namespace for tools | Set to your troubleshooting NS |
| Context | `--context` | Kube context | Set when not using current-context |
| Kubeconfig | `--kubeconfig` | Config path | Optional |
| Image | `-i` / `--image` | Container image | Default GHCR latest (resolved) |
| Resolve image | `--resolve-image` | Pin `:latest` to highest GHCR semver | `true` (default); `false` air-gap |
| Exec timeout | `--exec-timeout` | Per-exec deadline | `120s` default |
| Max output | `--max-output-bytes` | Captured stdout+stderr cap | `1048576` default |
| Insecure any pod | `--insecure-allow-any-pod` | Skip owner/managed checks | **Never** in shared clusters |

Inherited kubectl client flags (`--server`, `--token`, `--as`, certs, etc.) also
apply to `mcp`.

#### MCP flags

| Flag                       | Description                                                                                                                                                | Default                              |
| -------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------ |
| `--owner`                  | Owner label stamped on created resources; required for delete/exec authorization. Optional; defaults to `$USER` when unset. Required when `USER` is empty. | `$USER` when unset and `USER` is set |
| `--exec-timeout`           | Timeout for exec tools                                                                                                                                     | `120s`                               |
| `--max-output-bytes`       | Max stdout+stderr captured per exec                                                                                                                        | `1048576`                            |
| `--insecure-allow-any-pod` | Skip managed/owner label checks (dangerous)                                                                                                                | `false`                              |
| `--resolve-image`          | Resolve `:latest` to the highest semver tag on GHCR (falls back to configured image on error)                                                              | `true`                               |

#### Guardrails

- Pods created via MCP are labeled `kubectl-netdrill.io/managed=true` and
  `kubectl-netdrill.io/owner=<owner>`.
- Optional `ticketId` on create tools adds `kubectl-netdrill.io/ticket`;
  delete/exec require a matching ticket when present.
- Delete and exec are authorized by **labels on the live pod**, not by pod name
  alone—so another user's netdrill pod in the same namespace is not deletable
  from your session.
- Use `netdrill_list_managed_pods` to list pods for your owner before delete.

#### Server instructions

On MCP initialize, the server returns instructions that steer agents to:

1. Read `netdrill://container-tools` before inventing HTTPS/AWS/DNS commands.
2. Prefer `https-wrench`, `aws-probe`, and `doggo` via `netdrill_pod_exec`.
3. Pass `nodeSelector` / `serviceAccount` / `env` / `ports` / `hostNetwork` on
   create tools when needed.
4. Follow create → wait → exec → delete, and only touch owner-labeled pods.

#### CLI ↔ MCP parity

| Capability | CLI | MCP |
| ---------- | --- | --- |
| Persistent pod | `pod` | `netdrill_pod_create` |
| Ephemeral pod create | `run` (auto-delete on exit) | `netdrill_run_create` (no auto-delete; use `netdrill_run_cleanup`) |
| Interactive TTY attach | `run` / `debug` | **CLI only** |
| Non-interactive exec | `kubectl exec` / attach | `netdrill_pod_exec` / `netdrill_debug_exec` |
| `--node-selector` | yes | `nodeSelector` |
| `--service-account` | yes | `serviceAccount` |
| `--env` / `--port` / `--host-network` | yes | `env` / `ports` / `hostNetwork` |
| Deployment replicas/resources/labels | yes | `replicas` / `cpuRequest` / … / `labels` |
| Debug `--target` | yes | `targetContainer` on `netdrill_debug_add` |

#### MCP tools (v1)

Shared optional fields on most tools: `namespace` (defaults to MCP `-n`),
`ticketId` (ticket label for create/auth).

| Tool | Required | Optional | Purpose |
| ---- | -------- | -------- | ------- |
| `netdrill_pod_create` | — | `podName`, `nodeSelector`, `serviceAccount`, `env`, `ports`, `hostNetwork`, `namespace`, `ticketId` | Persistent troubleshooting pod |
| `netdrill_pod_delete` | `podName` | `namespace`, `ticketId` | Delete an authorized pod |
| `netdrill_pod_wait` | `podName` | `namespace`, `ticketId` | Wait until pod is Running |
| `netdrill_pod_exec` | `podName`, `command` | `containerName`, `namespace`, `ticketId` | Run command; return stdout/stderr; also mirrors command+output into the container's `kubectl logs` (PID 1 stdout, best-effort) |
| `netdrill_run_create` | — | same as pod create + `command` | Ephemeral-style pod (manual cleanup) |
| `netdrill_run_cleanup` | `podName` | `namespace`, `ticketId` | Delete ephemeral pod |
| `netdrill_deployment_create` | — | `name`, create fields above + `replicas`, `labels`, `cpuRequest`, `memoryRequest`, `cpuLimit`, `memoryLimit` | Create Deployment |
| `netdrill_deployment_delete` | `name` | `namespace`, `ticketId` | Delete Deployment |
| `netdrill_debug_add` | `podName` | `targetContainer`, `namespace`, `ticketId` | Add `netdrill-debug` ephemeral container |
| `netdrill_debug_exec` | `podName`, `command` | `namespace`, `ticketId` | Exec in ephemeral container (same log mirroring as `netdrill_pod_exec`) |
| `netdrill_list_managed_pods` | — | `namespace`, `ticketId` | List managed pods for this owner |

##### Examples

Pin a pod to a node and use IRSA:

```json
{
  "podName": "netdrill-on-flux",
  "nodeSelector": { "kubernetes.io/hostname": "flux" },
  "serviceAccount": "aws-client",
  "env": { "AWS_REGION": "eu-central-1" }
}
```

#### MCP resources and prompts (container tools)

| Name                            | Purpose                                                                          |
| ------------------------------- | -------------------------------------------------------------------------------- |
| `netdrill://container-tools`    | Markdown catalog of netdrill image CLIs and example `netdrill_pod_exec` commands |
| `netdrill_prompt_aws_in_pod`    | Workflow: pod + IRSA → exec `aws-probe` in cluster                               |
| `netdrill_prompt_https_in_pod`  | Workflow: pod → exec `https-wrench`                                              |
| `netdrill_prompt_network_check` | Workflow: pod → ping/curl/doggo to a target                                      |

</details>
