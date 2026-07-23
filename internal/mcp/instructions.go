package mcp

// AgentInstructions is returned in the MCP initialize result so clients can
// steer agents toward image CLIs and node placement without inventing workflows.
const AgentInstructions = `You are using kubectl-netdrill MCP to run troubleshooting pods in Kubernetes.

Before HTTPS/TLS, AWS, DNS, or connectivity work from a netdrill pod:
read the MCP resource netdrill://container-tools and follow its command patterns.

Image tools (prefer these via netdrill_pod_exec / netdrill_debug_exec;
do not reinvent with openssl/curl/awscli unless the catalog lacks the capability):
- https-wrench — HTTPS and TLS probes (cert expiry, endpoints, request YAML).
  Prefer https-wrench over openssl s_client/curl for cert and HTTPS checks
  from the cluster path.
- aws-probe — AWS identity and read-only AWS APIs from inside the pod
  (IRSA via create param serviceAccount). Do not use aws-probe MCP on the host
  unless the user asked for workstation AWS access.
- doggo (preferred) / nslookup for DNS; dig is not on PATH.

Create tool options (netdrill_pod_create / netdrill_run_create /
netdrill_deployment_create):
- nodeSelector — when the user names a node/host, use
  {"kubernetes.io/hostname":"<nodeName>"} (same as CLI
  --node-selector kubernetes.io/hostname=<nodeName>). Do not apply ad-hoc Pod
  YAML with nodeName to bypass netdrill.
- serviceAccount — set for EKS IRSA (IAM role via ServiceAccount).
- env, ports, hostNetwork — match CLI --env / --port / --host-network.
- deployment only: replicas, labels, cpuRequest, memoryRequest, cpuLimit,
  memoryLimit.

Image:
- Default image is ghcr.io/xenos76/netdrill. With --resolve-image (default),
  MCP pins :latest to the highest GHCR semver tag for this session.
  Do not invent or pin an older tag yourself unless the user asks.

Standard workflow: create → netdrill_pod_wait → netdrill_pod_exec →
netdrill_pod_delete (or run_cleanup). Use netdrill_list_managed_pods before delete.
Only operate on pods labeled for this session owner.`
