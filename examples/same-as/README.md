# `--same-as` Test Fixtures & Practice Guide

This directory contains Kubernetes manifests to set up a test environment covering **all operational settings and options** managed by the `kubectl-netdrill` `--same-as` and `--same-as-container` flags.

---

## 1. Test Workload Manifests

| File | Workload | Settings Covered |
| :--- | :--- | :--- |
| [`00-setup.yaml`](00-setup.yaml) | Namespace, ServiceAccount, ConfigMap, Secret | Namespace isolation (`netdrill-demo`), ServiceAccount token permissions, and configuration sources for `envFrom` and volume mounts. |
| [`01-standard-pod.yaml`](01-standard-pod.yaml) | Pod (`demo-web-pod`) | CPU & Memory requests/limits, container ports (`8080`, `8443`), direct `env`, `envFrom` (ConfigMap + Secret), safe volume mounts, `nodeSelector`, and `tolerations`. |
| [`02-advanced-scheduling-pod.yaml`](02-advanced-scheduling-pod.yaml) | Pod (`demo-scheduling-pod`) | Pod & Container `securityContext` (`runAsUser: 10001`, `runAsNonRoot: true`), `affinity` (node & pod anti-affinity), `topologySpreadConstraints`, custom `dnsConfig` & `dnsPolicy`, `emptyDir`, and `downwardAPI` volumes. |
| [`03-multi-container-pod.yaml`](03-multi-container-pod.yaml) | Multi-container Pod (`demo-multi-container`) | 3 distinct containers (`api-gateway`, `envoy-proxy`, `log-shipper`) for practicing `--same-as-container` targeting and shell autocompletion. |
| [`04-storage-and-exclusion-pod.yaml`](04-storage-and-exclusion-pod.yaml) | Pod (`demo-storage-pod`) & PVC | Verifies safe volume cloning: `ConfigMap` and `emptyDir` volumes are cloned, while `PersistentVolumeClaim` is safely excluded to avoid multi-attach conflicts. |
| [`05-production-deployment.yaml`](05-production-deployment.yaml) | Deployment (`demo-order-api`) | Deployment cloning (`replicas: 2`), pod template labels/annotations filtering (`deployment.kubernetes.io/*` stripped), resources, ports, and cross-kind cloning (Deployment → Pod). |
| [`test-same-as.sh`](test-same-as.sh) | Automated Test Script | Deploys all fixtures, tests every drill programmatically, asserts on expected cloning outcomes, and reports a pass/fail summary. |

---

## 2. Automated Execution or Manual Deployment

### Option A: Run the Automated Validation Script

To automatically deploy the fixtures, execute all validation drills against your cluster, and print a pass/fail report:

```bash
# Run all tests and keep resources for inspection:
./examples/same-as/test-same-as.sh

# Run tests and output unified diff comparisons of original vs cloned resources:
./examples/same-as/test-same-as.sh --compare

# Run tests, prompt/pause before deletion to inspect resources:
./examples/same-as/test-same-as.sh --clean --pause

# Clean up test resources upon completion:
./examples/same-as/test-same-as.sh --clean
```

### Option B: Manual Deployment & Hands-On Practice

Apply all resources to your test cluster:

```bash
kubectl apply -f examples/same-as/
```

Verify that the sample workloads are running:

```bash
kubectl get pods,deployments -n netdrill-demo
```

---

## 3. Hands-On Practice Scenarios

Ensure the `kubectl-netdrill` binary from this repository is in your `$PATH` or use `./dist/kubectl-netdrill`:

```bash
alias netdrill="./dist/kubectl-netdrill"
```

### Drill 1: Clone a Standard Pod

Clone operational settings from `demo-web-pod`. The troubleshooting pod runs netdrill image while copying ports, env, SA, and labels:

```bash
netdrill pod demo-web-pod-clone -n netdrill-demo --same-as demo-web-pod
```

Inspect the cloned pod in a second terminal:

```bash
kubectl get pod demo-web-pod-clone -n netdrill-demo -o yaml
```

Notice:
- Image is `ghcr.io/xenos76/netdrill:latest`.
- Cloned ports `8080` and `8443`.
- Cloned environment variables (`HTTP_PORT`, `SERVICE_NAME`) and `envFrom` (ConfigMap `app-config` and Secret `app-secret`).
- Mounted `/etc/app/config` and `/etc/app/secrets`.

---

### Drill 2: Cross-Namespace Cloning with Syntax Variations

Run netdrill in your current default namespace while cloning from the `netdrill-demo` namespace:

```bash
# Syntax: <namespace>/<name>
netdrill pod test-cross-ns --same-as netdrill-demo/demo-web-pod

# Syntax: <namespace>/<kind>/<name>
netdrill pod test-cross-ns-explicit --same-as netdrill-demo/pod/demo-web-pod
```

---

### Drill 3: Target a Specific Container in Multi-Container Workloads

The `demo-multi-container` pod has three containers: `api-gateway` (port 8080), `envoy-proxy` (ports 10000, 9901), and `log-shipper`.

**Default behavior (clones first container: `api-gateway`)**:
```bash
netdrill pod test-default-container -n netdrill-demo --same-as demo-multi-container
```

**Targeting the `envoy-proxy` container**:
```bash
netdrill pod demo-multi-container-clone -n netdrill-demo \
    --same-as demo-multi-container \
    --same-as-container envoy-proxy
```
Verify that the ports exposed on `demo-multi-container-clone` are `10000` and `9901`, and the volume mount is `proxy-cache`.

**Targeting the `log-shipper` container**:
```bash
netdrill pod test-logger-drill -n netdrill-demo \
    --same-as demo-multi-container \
    --same-as-container log-shipper
```

---

### Drill 4: Advanced Scheduling, SecurityContext, and Custom DNS

Clone `demo-scheduling-pod` to test affinity, non-root user replication, and DNS settings:

```bash
netdrill pod demo-scheduling-pod-clone -n netdrill-demo --same-as demo-scheduling-pod
```

Inspect the cloned pod:
```bash
kubectl get pod demo-scheduling-pod-clone -n netdrill-demo -o jsonpath='{.spec.securityContext}'
# Output shows runAsUser: 10001, runAsNonRoot: true

kubectl get pod demo-scheduling-pod-clone -n netdrill-demo -o jsonpath='{.spec.dnsConfig}'
# Output shows custom nameservers (1.1.1.1, 8.8.8.8) and searches

kubectl get pod demo-scheduling-pod-clone -n netdrill-demo -o jsonpath='{.spec.affinity}'
# Output shows nodeAffinity and podAntiAffinity rules copied intact
```

---

### Drill 5: Storage & Exclusion Verification (PVC Omitted)

Clone `demo-storage-pod`, which defines a PersistentVolumeClaim, ConfigMap, and EmptyDir:

```bash
netdrill pod demo-storage-pod-clone -n netdrill-demo --same-as demo-storage-pod
```

Verify volume mounts on the new pod:
```bash
kubectl get pod demo-storage-pod-clone -n netdrill-demo -o jsonpath='{.spec.volumes[*].name}'
```
- **Included**: `config-volume`, `temp-storage`.
- **Excluded**: `persistent-data` (the PVC was safely skipped to avoid ReadWriteOnce attach conflicts).

---

### Drill 6: Deployment-to-Deployment Cloning

Clone a production deployment. Replicas (`2`), pod template resources, labels, and anti-affinity are cloned:

```bash
netdrill deployment demo-order-api-clone -n netdrill-demo --same-as demo-order-api
```

Verify the cloned deployment:
```bash
kubectl get deployment demo-order-api-clone -n netdrill-demo
# Replicas: 2/2

kubectl get deployment demo-order-api-clone -n netdrill-demo -o jsonpath='{.spec.template.spec.containers[0].resources}'
# Requests: cpu 250m, memory 256Mi; Limits: cpu 500m, memory 512Mi
```

---

### Drill 7: Deployment-to-Pod Cloning

Clone a Deployment into a single interactive troubleshooting Pod:

```bash
netdrill pod demo-order-api-pod-clone -n netdrill-demo --same-as deployment/demo-order-api
```

The pod inherits the deployment template's labels, service account, ports, env, and resources.

---

### Drill 8: Overriding Cloned Settings with CLI Flags

Explicit flags always take precedence over cloned values:

```bash
netdrill pod demo-web-pod-override-clone -n netdrill-demo \
    --same-as demo-web-pod \
    --cpu-request 50m \
    --cpu-limit 100m \
    --port 3000 \
    --env HTTP_PORT=3000 \
    --env DEBUG=true \
    --labels environment=testing-override
```

Verify:
- CPU request is `50m` (overriding `150m`).
- Port is `3000` (overriding `8080, 8443`).
- `HTTP_PORT` is `3000` (overridden from `8080`), and `DEBUG=true` is appended.
- Label `environment` is `testing-override` (overriding `staging`).

---

### Drill 9: Shell Autocompletion

If shell completion is loaded, test dynamic tab completion:

```bash
# Proposes workloads in current namespace:
netdrill pod debug --same-as <TAB>

# Proposes workloads in netdrill-demo:
netdrill pod debug -n netdrill-demo --same-as <TAB>

# Proposes containers in the selected pod:
netdrill pod debug -n netdrill-demo --same-as demo-multi-container --same-as-container <TAB>
# Shows: api-gateway, envoy-proxy, log-shipper
```

---

## 4. Cleanup

When done testing, remove all sample workloads:

```bash
kubectl delete -f examples/same-as/
```
