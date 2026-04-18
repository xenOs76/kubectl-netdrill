# kubectl-netdrill

[![Go Report Card](https://goreportcard.com/badge/github.com/xenos76/kubectl-netdrill)](https://goreportcard.com/badge/github.com/xenos76/kubectl-netdrill)

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

| Command                        | Description                                                   |
| ------------------------------ | ------------------------------------------------------------- |
| `kubectl netdrill run [name]`  | Create a temporary troubleshooting pod (auto-deleted on exit) |
| `kubectl netdrill pod [name]`  | Create a persistent troubleshooting pod                       |
| `kubectl netdrill debug <pod>` | Inject ephemeral container into a running pod                 |

### kubectl netdrill run

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

### kubectl netdrill pod

Create a persistent troubleshooting pod that remains running after you exit
(unlike `run` which auto-deletes):

```bash
kubectl netdrill pod my-persistent-troubleshooter
```

#### Options

| Flag                | Description                                          | Default                |
| ------------------- | ---------------------------------------------------- | ---------------------- |
| `--service-account` | ServiceAccount to use for the pod                    |                        |
| `--port`            | Ports to expose on the container (e.g., `--port 80`) |                        |
| `--env`             | Environment variables (e.g., `--env KEY=VALUE`)      |                        |
| `--host-network`    | Use the host's network namespace                     | `false`                |
| `--node-selector`   | Node labels for pod scheduling                       |                        |
| `--image`           | `-i`                                                 | Container image to use |

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

Create a pod with a specific IAM role in Kubernetes/EKS:

```bash
kubectl-netdrill pod my-s3-client \
    --env AWS_REGION='eu-central-1' \
    --env AWS_ROLE_ARN='arn:aws:iam::AWS_ACCOUNT_ID:role/IAMRoleName' \
    --env AWS_WEB_IDENTITY_TOKEN_FILE='/run/secrets/kubernetes.io/serviceaccount/token' \
    --service-account aws-client
```

Attach to an existing persistent pod:

```bash
kubectl exec -it my-persistent-troubleshooter -n default -- /bin/bash
```

### kubectl netdrill debug

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

| Flag              | Short | Description                                                    | Default                           |
| ----------------- | ----- | -------------------------------------------------------------- | --------------------------------- |
| `--target`        |       | Container name to share PID namespace with                     |                                   |
| `--node-selector` |       | Node labels for pod scheduling (e.g. `kubernetes.io/os=linux`) |                                   |
| `--image`         | `-i`  | Container image to use                                         | `ghcr.io/xenos76/netdrill:latest` |

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
