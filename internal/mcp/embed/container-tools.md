# Netdrill container tools

The default netdrill image (`ghcr.io/xenos76/netdrill`) ships CLIs and utilities for
in-cluster troubleshooting. When `--resolve-image=true`, MCP startup resolves untagged/`:latest` GHCR references to the
highest semver tag (for example `ghcr.io/xenos76/netdrill:v0.1.2`) so pods pull a concrete release.
If resolution fails, MCP uses the configured image unchanged.
Use **netdrill_pod_create** (pass **`serviceAccount`** when AWS/IRSA access is
needed; optional **`nodeSelector`**, **`env`**, **`ports`**, **`hostNetwork`**),
**netdrill_pod_wait**, then **netdrill_pod_exec** with the command arrays below.
Credentials come from the pod (for example EKS IRSA), not from the MCP host.

For direct AWS API access from your workstation, use the separate **aws-probe mcp**
server instead of re-implementing AWS tools here.

## Bundled CLIs (exec in netdrill container)

| Tool | Path | Purpose | Example `command` for netdrill_pod_exec |
|------|------|---------|----------------------------------------|
| aws-probe | `/usr/local/bin/aws-probe` | AWS read-only CLI (S3, SQS, Secrets, MSK, SNS, CloudFront, whoami) | `["aws-probe","whoami"]`, `["aws-probe","s3","list-buckets"]` |
| https-wrench | `/usr/local/bin/https-wrench` | HTTPS/TLS probes from inside the cluster | `["https-wrench","certinfo","--tls-endpoint","example.com:443"]` (prefer over openssl/curl) |

Upstream: [aws-probe](https://github.com/xenos76/aws-probe), [https-wrench](https://github.com/xenos76/https-wrench).

## Create options

Pass these JSON fields on `netdrill_pod_create` / `netdrill_run_create` /
`netdrill_deployment_create` (same meaning as the CLI flags):

| Param | CLI flag | Notes |
|-------|----------|-------|
| `nodeSelector` | `--node-selector` | e.g. `{"kubernetes.io/hostname":"<nodeName>"}` |
| `serviceAccount` | `--service-account` | Required for EKS IRSA |
| `env` | `--env` | Map of environment variables |
| `ports` | `--port` | Array of container ports |
| `hostNetwork` | `--host-network` | Host network namespace |
| `replicas` / `labels` / `cpuRequest` / `memoryRequest` / `cpuLimit` / `memoryLimit` | matching deployment flags | Deployment create only |

Do not apply ad-hoc Pod YAML with `nodeName` to bypass netdrill create tools.

## Network and shell utilities

DNS in the image: **doggo** (preferred) and **nslookup**. **dig** is not on `PATH`.

| Tool | Example exec |
|------|----------------|
| ping | `["ping","-c","3","1.1.1.1"]` |
| curl | `["curl","-sS","-o","/dev/null","-w","%{http_code}","https://example.com"]` |
| doggo | `["doggo","example.com","NS"]` |
| nslookup | `["nslookup","example.com"]` |
| iperf3 | `["iperf3","-s"]` (server; open port via pod spec if needed) |
| tcpdump | `["tcpdump","-i","any","-c","10"]` |
| nmap | `["nmap","-sn","10.0.0.0/24"]` |
| wget | `["wget","-qO-","https://example.com"]` |
| jq | `["jq","-n","{}"]` |

## Port checks (nmap)

Check whether a specific port is open and identify the service. Replace `TARGET` with a host
or IP.

- HTTP (TCP 80): `["nmap","-p","80","-sT","-sV","TARGET"]`
- DNS (UDP 53): `["nmap","-sU","-p","53","-sV","TARGET"]`

`-sT` uses a TCP connect scan; `-sU` probes UDP. `-sV` reports the service (for example
`http` or `domain`). UDP scans may require extra pod capabilities depending on the cluster.

## DNS queries

Recursive lookup (cluster or public resolver):

- `["doggo","example.com","NS"]`
- `["doggo","example.com","SOA","@10.43.0.10"]` (CoreDNS; namespace may vary)
- `["nslookup","-type=NS","example.com"]`

Authoritative lookup (query the zone nameserver with recursion disabled).
Resolve NS glue first, then query that IP. Prefer `@tcp://` when UDP port 53 is flaky:

- SOA: `["sh","-c","doggo example.com SOA @tcp://NS_IP --rd=false"]`
- NS: `["sh","-c","doggo example.com NS @tcp://NS_IP --rd=false"]`

Replace `NS_IP` with the authoritative nameserver address (for example from NS A/AAAA glue).

DNS over HTTPS (DoH) and DNS over TLS (DoT) via doggo transport URLs (`@https://`, `@tls://`):

- DoH (Cloudflare): `["doggo","example.com","A","@https://cloudflare-dns.com/dns-query"]`
- DoH (Google): `["doggo","example.com","A","@https://dns.google/dns-query"]`
- DoT (Cloudflare): `["doggo","example.com","A","@tls://1.1.1.1","--tls-hostname","one.one.one.one"]`
- DoT (Quad9): `["doggo","example.com","A","@tls://9.9.9.9","--tls-hostname","dns.quad9.net"]`

When the DoT resolver is an IP, set `--tls-hostname` to the provider hostname for certificate
verification. Use `--skip-hostname-verification` only for deliberate test setups.

## Typical workflow

1. **netdrill_pod_create** — set `nodeSelector` when pinning to a named node; set
   `serviceAccount` when the pod must assume an IAM role (IRSA); use `env` /
   `ports` / `hostNetwork` as needed.
2. **netdrill_pod_wait** — wait until phase Running.
3. **netdrill_pod_exec** — run a row from the tables above (prefer **https-wrench**
   for TLS/HTTPS and **aws-probe** for AWS).
4. **netdrill_pod_delete** — cleanup when finished.
