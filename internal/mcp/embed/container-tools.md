# Netdrill container tools

The default netdrill image (`ghcr.io/xenos76/netdrill`) ships CLIs and utilities for
in-cluster troubleshooting. The MCP server pins `:latest` to the highest semver tag on GHCR
at startup (for example `ghcr.io/xenos76/netdrill:v0.1.2`) so pods pull a concrete release.
Use **netdrill_pod_create** (with a ServiceAccount when
AWS access is needed), **netdrill_pod_wait**, then **netdrill_pod_exec** with the command
arrays below. Credentials come from the pod (for example EKS IRSA), not from the MCP host.

For direct AWS API access from your workstation, use the separate **aws-probe mcp**
server instead of re-implementing AWS tools here.

## Bundled CLIs (exec in netdrill container)

| Tool | Path | Purpose | Example `command` for netdrill_pod_exec |
|------|------|---------|----------------------------------------|
| aws-probe | `/usr/local/bin/aws-probe` | AWS read-only CLI (S3, SQS, Secrets, MSK, SNS, CloudFront, whoami) | `["aws-probe","whoami"]`, `["aws-probe","s3","list-buckets"]` |
| https-wrench | `/usr/local/bin/https-wrench` | HTTPS/TLS probes from inside the cluster | `["https-wrench","--help"]` (see https-wrench docs for request YAML) |

Upstream: [aws-probe](https://github.com/xenos76/aws-probe), [https-wrench](https://github.com/xenos76/https-wrench).

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

1. **netdrill_pod_create** — set `serviceAccountName` when the pod must assume an IAM role (IRSA).
2. **netdrill_pod_wait** — wait until phase Running.
3. **netdrill_pod_exec** — run a row from the tables above.
4. **netdrill_pod_delete** — cleanup when finished.
