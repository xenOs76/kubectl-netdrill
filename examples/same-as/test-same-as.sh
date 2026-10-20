#!/usr/bin/env bash
# ==============================================================================
# test-same-as.sh: Automated resource deployment and --same-as feature validation
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
NAMESPACE="netdrill-demo"

# Colors for pretty terminal output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

PASSED_COUNT=0
FAILED_COUNT=0
CLEANUP_ON_EXIT=false
DO_COMPARE=false
DO_PAUSE=false
FORCE=false

# Cloned resource definitions following original-name-clone pattern
CLONED_PODS=(
    "demo-web-pod-clone"
    "demo-multi-container-clone"
    "demo-scheduling-pod-clone"
    "demo-storage-pod-clone"
    "demo-order-api-pod-clone"
    "demo-web-pod-override-clone"
    # Legacy names for backward-compatible cleanup
    "test-web-drill"
    "test-envoy-drill"
    "test-logger-drill"
    "test-advanced-drill"
    "test-storage-drill"
    "test-order-pod"
    "test-override-drill"
)

CLONED_DEPS=(
    "demo-order-api-clone"
    "test-orders-dep"
)

# Parse CLI options
for arg in "$@"; do
    case "${arg}" in
        --clean)
            CLEANUP_ON_EXIT=true
            ;;
        --compare|-c)
            DO_COMPARE=true
            ;;
        --pause|-p)
            DO_PAUSE=true
            ;;
        --force|-f)
            FORCE=true
            ;;
        --help|-h)
            printf "Usage: %s [--clean] [--compare|-c] [--pause|-p] [--force|-f]\n" "$0"
            printf "  --clean       Clean up test resources upon completion\n"
            printf "  --compare, -c Compare original and cloned resources (diff)\n"
            printf "  --pause, -p   Pause for manual inspection before cleanup\n"
            printf "  --force, -f   Skip interactive prompts during cleanup\n"
            exit 0
            ;;
    esac
done

log_info() {
    printf "${BLUE}[INFO]${NC} %s\n" "$*"
}

log_step() {
    printf "\n${BOLD}${CYAN}==>${NC} ${BOLD}%s${NC}\n" "$*"
}

log_pass() {
    printf "  ${GREEN}✔ [PASS]${NC} %s\n" "$*"
    PASSED_COUNT=$((PASSED_COUNT + 1))
}

log_fail() {
    printf "  ${RED}✖ [FAIL]${NC} %s\n" "$*"
    FAILED_COUNT=$((FAILED_COUNT + 1))
}

log_warn() {
    printf "${YELLOW}[WARN]${NC} %s\n" "$*"
}

compare_pair() {
    local kind="$1"
    local orig="$2"
    local clone="$3"
    printf "\n${BOLD}${CYAN}--- Comparing %s: %s (original) <--> %s (clone) ---${NC}\n" "${kind}" "${orig}" "${clone}"
    local filter_py='
import sys, json
try:
    data = json.load(sys.stdin)
    data.pop("status", None)
    m = data.get("metadata", {})
    for k in ["uid", "resourceVersion", "creationTimestamp", "generation", "managedFields", "selfLink"]:
        m.pop(k, None)
    print(json.dumps(data, indent=2))
except Exception:
    pass
'
    if command -v python3 >/dev/null 2>&1; then
        diff -u \
            <(kubectl get "${kind}" "${orig}" -n "${NAMESPACE}" -o json 2>/dev/null | python3 -c "${filter_py}") \
            <(kubectl get "${kind}" "${clone}" -n "${NAMESPACE}" -o json 2>/dev/null | python3 -c "${filter_py}") || true
    else
        diff -u \
            <(kubectl get "${kind}" "${orig}" -n "${NAMESPACE}" -o yaml 2>/dev/null) \
            <(kubectl get "${kind}" "${clone}" -n "${NAMESPACE}" -o yaml 2>/dev/null) || true
    fi
}

compare_deployment_to_pod() {
    local dep="$1"
    local pod="$2"
    printf "\n${BOLD}${CYAN}--- Comparing Deployment template vs Pod: %s (deployment) <--> %s (pod) ---${NC}\n" "${dep}" "${pod}"
    if command -v python3 >/dev/null 2>&1; then
        diff -u \
            <(kubectl get deployment "${dep}" -n "${NAMESPACE}" -o jsonpath='{.spec.template.spec}' 2>/dev/null | python3 -m json.tool 2>/dev/null) \
            <(kubectl get pod "${pod}" -n "${NAMESPACE}" -o jsonpath='{.spec}' 2>/dev/null | python3 -m json.tool 2>/dev/null) || true
    else
        diff -u \
            <(kubectl get deployment "${dep}" -n "${NAMESPACE}" -o yaml 2>/dev/null) \
            <(kubectl get pod "${pod}" -n "${NAMESPACE}" -o yaml 2>/dev/null) || true
    fi
}

compare_all_resources() {
    log_step "Comparing Original vs Cloned Resources in '${NAMESPACE}'..."
    compare_pair pod demo-web-pod demo-web-pod-clone
    compare_pair pod demo-multi-container demo-multi-container-clone
    compare_pair pod demo-scheduling-pod demo-scheduling-pod-clone
    compare_pair pod demo-storage-pod demo-storage-pod-clone
    compare_pair deployment demo-order-api demo-order-api-clone
    compare_deployment_to_pod demo-order-api demo-order-api-pod-clone
    compare_pair pod demo-web-pod demo-web-pod-override-clone
}

cleanup() {
    if [[ "${CLEANUP_ON_EXIT}" == "true" ]]; then
        if [[ "${FORCE}" != "true" && -t 0 ]]; then
            printf "\n${BOLD}${YELLOW}[CLEANUP PAUSE]${NC} Cloned resources are active in namespace '%s'.\n" "${NAMESPACE}"
            printf "  Press ${BOLD}'c'${NC} to view full diff comparison now\n"
            printf "  Press ${BOLD}[Enter]${NC} to proceed with deletion\n"
            printf "  Press ${BOLD}Ctrl+C${NC} to cancel cleanup and keep all resources\n"
            read -r -p "Select action [c/Enter/Ctrl+C]: " ACTION || true
            if [[ "${ACTION:-}" =~ ^[cC]$ ]]; then
                compare_all_resources
                read -r -p "Finished comparison. Press [Enter] to delete resources, or Ctrl+C to keep them: " _ || true
            fi
        elif [[ "${DO_PAUSE}" == "true" ]]; then
            printf "\n${BOLD}${YELLOW}[PAUSE]${NC} Pausing before deletion as requested by --pause.\n"
            read -r -p "Press [Enter] to proceed with cleanup, or Ctrl+C to keep resources: " _ || true
        fi

        log_step "Cleaning up netdrill demo resources..."
        kubectl delete pod "${CLONED_PODS[@]}" -n "${NAMESPACE}" --ignore-not-found=true --wait=false >/dev/null 2>&1 || true
        kubectl delete deployment "${CLONED_DEPS[@]}" -n "${NAMESPACE}" --ignore-not-found=true --wait=false >/dev/null 2>&1 || true
        kubectl delete -f "${SCRIPT_DIR}" --ignore-not-found=true >/dev/null 2>&1 || true
        log_info "Cleanup complete."
    fi
}

trap cleanup EXIT

# ------------------------------------------------------------------------------
# 1. Prerequisite Checks
# ------------------------------------------------------------------------------
log_step "1. Checking prerequisites..."

if ! command -v kubectl >/dev/null 2>&1; then
    log_fail "kubectl is not installed or not in PATH."
    exit 1
fi

if ! kubectl cluster-info >/dev/null 2>&1; then
    log_fail "Cannot connect to a Kubernetes cluster via kubectl. Please check your kubeconfig."
    exit 1
fi
log_pass "Kubernetes cluster is reachable."

NETDRILL_BIN="${REPO_ROOT}/dist/kubectl-netdrill"
if [[ ! -x "${NETDRILL_BIN}" ]]; then
    log_info "Netdrill binary not found at ${NETDRILL_BIN}. Building via build.sh..."
    (cd "${REPO_ROOT}" && ./build.sh)
fi

if [[ ! -x "${NETDRILL_BIN}" ]]; then
    log_fail "Failed to build or locate kubectl-netdrill binary at ${NETDRILL_BIN}"
    exit 1
fi
log_pass "Using binary: ${NETDRILL_BIN}"

# ------------------------------------------------------------------------------
# 2. Deploy Fixtures
# ------------------------------------------------------------------------------
log_step "2. Deploying test fixtures in namespace '${NAMESPACE}'..."
kubectl apply -f "${SCRIPT_DIR}/00-setup.yaml" >/dev/null
kubectl apply -f "${SCRIPT_DIR}/01-standard-pod.yaml" >/dev/null
kubectl apply -f "${SCRIPT_DIR}/02-advanced-scheduling-pod.yaml" >/dev/null
kubectl apply -f "${SCRIPT_DIR}/03-multi-container-pod.yaml" >/dev/null
kubectl apply -f "${SCRIPT_DIR}/04-storage-and-exclusion-pod.yaml" >/dev/null
kubectl apply -f "${SCRIPT_DIR}/05-production-deployment.yaml" >/dev/null

log_info "Waiting for target workloads to become ready..."
kubectl wait --for=condition=Ready pod/demo-web-pod -n "${NAMESPACE}" --timeout=60s >/dev/null
kubectl wait --for=condition=Ready pod/demo-scheduling-pod -n "${NAMESPACE}" --timeout=60s >/dev/null
kubectl wait --for=condition=Ready pod/demo-multi-container -n "${NAMESPACE}" --timeout=60s >/dev/null
kubectl rollout status deployment/demo-order-api -n "${NAMESPACE}" --timeout=60s >/dev/null
log_pass "All test workloads are ready."

# ------------------------------------------------------------------------------
# 3. Test Drill 1: Standard Pod Cloning
# ------------------------------------------------------------------------------
log_step "3. Testing Standard Pod Cloning (--same-as demo-web-pod)..."
kubectl delete pod demo-web-pod-clone -n "${NAMESPACE}" --ignore-not-found=true --wait=true >/dev/null 2>&1 || true

"${NETDRILL_BIN}" pod demo-web-pod-clone -n "${NAMESPACE}" --same-as demo-web-pod >/dev/null 2>&1 &
WAIT_PID=$!
sleep 3
kill "${WAIT_PID}" 2>/dev/null || true

# Assertions
IMAGE=$(kubectl get pod demo-web-pod-clone -n "${NAMESPACE}" -o jsonpath='{.spec.containers[0].image}')
if [[ "${IMAGE}" == *"netdrill"* ]]; then
    log_pass "Image retained netdrill default: ${IMAGE}"
else
    log_fail "Expected netdrill image, got: ${IMAGE}"
fi

SA=$(kubectl get pod demo-web-pod-clone -n "${NAMESPACE}" -o jsonpath='{.spec.serviceAccountName}')
if [[ "${SA}" == "netdrill-tester-sa" ]]; then
    log_pass "ServiceAccount cloned: ${SA}"
else
    log_fail "Expected ServiceAccount netdrill-tester-sa, got: ${SA}"
fi

PORTS=$(kubectl get pod demo-web-pod-clone -n "${NAMESPACE}" -o jsonpath='{.spec.containers[0].ports[*].containerPort}')
if [[ "${PORTS}" =~ 8080 && "${PORTS}" =~ 8443 ]]; then
    log_pass "Container ports cloned: ${PORTS}"
else
    log_fail "Expected ports 8080 and 8443, got: ${PORTS}"
fi

CONFIG_MOUNT=$(kubectl get pod demo-web-pod-clone -n "${NAMESPACE}" -o jsonpath='{.spec.containers[0].volumeMounts[?(@.name=="config-volume")].mountPath}')
if [[ "${CONFIG_MOUNT}" == "/etc/app/config" ]]; then
    log_pass "Safe ConfigMap volume mount cloned at ${CONFIG_MOUNT}"
else
    log_fail "Expected mount /etc/app/config, got: ${CONFIG_MOUNT}"
fi

# ------------------------------------------------------------------------------
# 4. Test Drill 2: Multi-Container Selection (--same-as-container)
# ------------------------------------------------------------------------------
log_step "4. Testing Multi-Container Targeting (--same-as-container envoy-proxy)..."
kubectl delete pod demo-multi-container-clone -n "${NAMESPACE}" --ignore-not-found=true --wait=true >/dev/null 2>&1 || true

"${NETDRILL_BIN}" pod demo-multi-container-clone -n "${NAMESPACE}" --same-as demo-multi-container --same-as-container envoy-proxy >/dev/null 2>&1 &
WAIT_PID=$!
sleep 3
kill "${WAIT_PID}" 2>/dev/null || true

ENVOY_PORTS=$(kubectl get pod demo-multi-container-clone -n "${NAMESPACE}" -o jsonpath='{.spec.containers[0].ports[*].containerPort}')
if [[ "${ENVOY_PORTS}" =~ 10000 && "${ENVOY_PORTS}" =~ 9901 ]]; then
    log_pass "Cloned envoy-proxy specific ports (10000, 9901): ${ENVOY_PORTS}"
else
    log_fail "Expected ports 10000 and 9901 for envoy-proxy, got: ${ENVOY_PORTS}"
fi

ENVOY_ROLE=$(kubectl get pod demo-multi-container-clone -n "${NAMESPACE}" -o jsonpath='{.spec.containers[0].env[?(@.name=="SERVICE_ROLE")].value}')
if [[ "${ENVOY_ROLE}" == "sidecar-proxy" ]]; then
    log_pass "Cloned envoy-proxy specific environment variable: SERVICE_ROLE=${ENVOY_ROLE}"
else
    log_fail "Expected SERVICE_ROLE=sidecar-proxy, got: ${ENVOY_ROLE}"
fi

# ------------------------------------------------------------------------------
# 5. Test Drill 3: Advanced Scheduling, SecurityContext, & DNS
# ------------------------------------------------------------------------------
log_step "5. Testing Advanced Scheduling, SecurityContext, and DNS..."
kubectl delete pod demo-scheduling-pod-clone -n "${NAMESPACE}" --ignore-not-found=true --wait=true >/dev/null 2>&1 || true

"${NETDRILL_BIN}" pod demo-scheduling-pod-clone -n "${NAMESPACE}" --same-as demo-scheduling-pod >/dev/null 2>&1 &
WAIT_PID=$!
sleep 3
kill "${WAIT_PID}" 2>/dev/null || true

RUN_AS_USER=$(kubectl get pod demo-scheduling-pod-clone -n "${NAMESPACE}" -o jsonpath='{.spec.securityContext.runAsUser}')
RUN_AS_NON_ROOT=$(kubectl get pod demo-scheduling-pod-clone -n "${NAMESPACE}" -o jsonpath='{.spec.securityContext.runAsNonRoot}')
if [[ "${RUN_AS_USER}" == "10001" && "${RUN_AS_NON_ROOT}" == "true" ]]; then
    log_pass "Pod securityContext cloned: runAsUser=${RUN_AS_USER}, runAsNonRoot=${RUN_AS_NON_ROOT}"
else
    log_fail "Expected runAsUser=10001, runAsNonRoot=true; got: user=${RUN_AS_USER}, nonRoot=${RUN_AS_NON_ROOT}"
fi

DNS_POLICY=$(kubectl get pod demo-scheduling-pod-clone -n "${NAMESPACE}" -o jsonpath='{.spec.dnsPolicy}')
DNS_NS=$(kubectl get pod demo-scheduling-pod-clone -n "${NAMESPACE}" -o jsonpath='{.spec.dnsConfig.nameservers[*]}')
if [[ "${DNS_POLICY}" == "ClusterFirstWithHostNet" && "${DNS_NS}" == *"1.1.1.1"* ]]; then
    log_pass "DNS Policy and custom nameservers cloned: policy=${DNS_POLICY}, nameservers=${DNS_NS}"
else
    log_fail "Expected ClusterFirstWithHostNet and 1.1.1.1, got: policy=${DNS_POLICY}, ns=${DNS_NS}"
fi

NODE_AFFINITY=$(kubectl get pod demo-scheduling-pod-clone -n "${NAMESPACE}" -o jsonpath='{.spec.affinity.nodeAffinity}')
if [[ -n "${NODE_AFFINITY}" ]]; then
    log_pass "NodeAffinity rules cloned successfully."
else
    log_fail "NodeAffinity rules were not cloned."
fi

# ------------------------------------------------------------------------------
# 6. Test Drill 4: Storage Safety (PVC Excluded, Safe Volumes Cloned)
# ------------------------------------------------------------------------------
log_step "6. Testing Storage Safety & PVC Exclusion..."
kubectl delete pod demo-storage-pod-clone -n "${NAMESPACE}" --ignore-not-found=true --wait=true >/dev/null 2>&1 || true

"${NETDRILL_BIN}" pod demo-storage-pod-clone -n "${NAMESPACE}" --same-as demo-storage-pod >/dev/null 2>&1 &
WAIT_PID=$!
sleep 3
kill "${WAIT_PID}" 2>/dev/null || true

VOLUMES=$(kubectl get pod demo-storage-pod-clone -n "${NAMESPACE}" -o jsonpath='{.spec.volumes[*].name}')
if [[ "${VOLUMES}" =~ "config-volume" && "${VOLUMES}" =~ "temp-storage" ]]; then
    log_pass "Safe volumes cloned: config-volume, temp-storage"
else
    log_fail "Expected config-volume and temp-storage in volumes, got: ${VOLUMES}"
fi

if [[ ! "${VOLUMES}" =~ "persistent-data" ]]; then
    log_pass "PVC 'persistent-data' correctly excluded to prevent multi-attach conflicts."
else
    log_fail "PVC was unexpectedly cloned in volumes: ${VOLUMES}"
fi

# ------------------------------------------------------------------------------
# 7. Test Drill 5: Deployment-to-Deployment Cloning
# ------------------------------------------------------------------------------
log_step "7. Testing Deployment-to-Deployment Cloning..."
kubectl delete deployment demo-order-api-clone -n "${NAMESPACE}" --ignore-not-found=true --wait=true >/dev/null 2>&1 || true

"${NETDRILL_BIN}" deployment demo-order-api-clone -n "${NAMESPACE}" --same-as demo-order-api >/dev/null

REPLICAS=$(kubectl get deployment demo-order-api-clone -n "${NAMESPACE}" -o jsonpath='{.spec.replicas}')
if [[ "${REPLICAS}" == "2" ]]; then
    log_pass "Deployment replicas cloned: ${REPLICAS}"
else
    log_fail "Expected 2 replicas, got: ${REPLICAS}"
fi

CPU_REQ=$(kubectl get deployment demo-order-api-clone -n "${NAMESPACE}" -o jsonpath='{.spec.template.spec.containers[0].resources.requests.cpu}')
if [[ "${CPU_REQ}" == "250m" ]]; then
    log_pass "Deployment template CPU request cloned: ${CPU_REQ}"
else
    log_fail "Expected CPU request 250m, got: ${CPU_REQ}"
fi

# ------------------------------------------------------------------------------
# 8. Test Drill 6: Deployment-to-Pod Cross-Kind Cloning
# ------------------------------------------------------------------------------
log_step "8. Testing Deployment-to-Pod Cross-Kind Cloning..."
kubectl delete pod demo-order-api-pod-clone -n "${NAMESPACE}" --ignore-not-found=true --wait=true >/dev/null 2>&1 || true

"${NETDRILL_BIN}" pod demo-order-api-pod-clone -n "${NAMESPACE}" --same-as deployment/demo-order-api >/dev/null 2>&1 &
WAIT_PID=$!
sleep 3
kill "${WAIT_PID}" 2>/dev/null || true

POD_APP_LABEL=$(kubectl get pod demo-order-api-pod-clone -n "${NAMESPACE}" -o jsonpath='{.metadata.labels.app}')
if [[ "${POD_APP_LABEL}" == "order-api" ]]; then
    log_pass "Pod successfully cloned template labels from Deployment: app=${POD_APP_LABEL}"
else
    log_fail "Expected app=order-api label on pod, got: ${POD_APP_LABEL}"
fi

# ------------------------------------------------------------------------------
# 9. Test Drill 7: Flag Overrides Precedence
# ------------------------------------------------------------------------------
log_step "9. Testing Explicit Flag Overrides..."
kubectl delete pod demo-web-pod-override-clone -n "${NAMESPACE}" --ignore-not-found=true --wait=true >/dev/null 2>&1 || true

"${NETDRILL_BIN}" pod demo-web-pod-override-clone -n "${NAMESPACE}" \
    --same-as demo-web-pod \
    --cpu-request 50m \
    --port 3000 \
    --env HTTP_PORT=3000 \
    --env EXTRA_KEY=custom-value \
    --labels environment=override >/dev/null 2>&1 &
WAIT_PID=$!
sleep 3
kill "${WAIT_PID}" 2>/dev/null || true

OVERRIDE_CPU=$(kubectl get pod demo-web-pod-override-clone -n "${NAMESPACE}" -o jsonpath='{.spec.containers[0].resources.requests.cpu}')
if [[ "${OVERRIDE_CPU}" == "50m" ]]; then
    log_pass "CPU request override applied: ${OVERRIDE_CPU} (overrode 150m)"
else
    log_fail "Expected CPU request 50m, got: ${OVERRIDE_CPU}"
fi

OVERRIDE_PORT=$(kubectl get pod demo-web-pod-override-clone -n "${NAMESPACE}" -o jsonpath='{.spec.containers[0].ports[0].containerPort}')
if [[ "${OVERRIDE_PORT}" == "3000" ]]; then
    log_pass "Container port override applied: ${OVERRIDE_PORT} (overrode 8080/8443)"
else
    log_fail "Expected containerPort 3000, got: ${OVERRIDE_PORT}"
fi

OVERRIDE_ENV=$(kubectl get pod demo-web-pod-override-clone -n "${NAMESPACE}" -o jsonpath='{.spec.containers[0].env[?(@.name=="HTTP_PORT")].value}')
APPENDED_ENV=$(kubectl get pod demo-web-pod-override-clone -n "${NAMESPACE}" -o jsonpath='{.spec.containers[0].env[?(@.name=="EXTRA_KEY")].value}')
if [[ "${OVERRIDE_ENV}" == "3000" && "${APPENDED_ENV}" == "custom-value" ]]; then
    log_pass "Environment variables merged and overridden correctly: HTTP_PORT=${OVERRIDE_ENV}, EXTRA_KEY=${APPENDED_ENV}"
else
    log_fail "Expected HTTP_PORT=3000 and EXTRA_KEY=custom-value, got: HTTP_PORT=${OVERRIDE_ENV}, EXTRA_KEY=${APPENDED_ENV}"
fi

OVERRIDE_LABEL=$(kubectl get pod demo-web-pod-override-clone -n "${NAMESPACE}" -o jsonpath='{.metadata.labels.environment}')
if [[ "${OVERRIDE_LABEL}" == "override" ]]; then
    log_pass "Label override applied: environment=${OVERRIDE_LABEL}"
else
    log_fail "Expected environment=override, got: ${OVERRIDE_LABEL}"
fi

# ------------------------------------------------------------------------------
# Summary & Comparison
# ------------------------------------------------------------------------------
printf "\n%b\n" "${BOLD}==============================================================================${NC}"
printf "%b\n" "${BOLD}Validation Summary:${NC}"
printf "  %b %d\n" "${GREEN}Passed:${NC}" "${PASSED_COUNT}"
printf "  %b %d\n" "${RED}Failed:${NC}" "${FAILED_COUNT}"
printf "%b\n" "${BOLD}==============================================================================${NC}"

if [[ "${FAILED_COUNT}" -gt 0 ]]; then
    exit 1
fi

log_info "All --same-as features validated successfully against the cluster!"

if [[ "${DO_COMPARE}" == "true" ]]; then
    compare_all_resources
fi

if [[ "${CLEANUP_ON_EXIT}" != "true" ]]; then
    log_info "Test resources are kept in namespace '${NAMESPACE}' for inspection."
    printf "\n${BOLD}Available cloned resources for comparison:${NC}\n"
    printf "  - Pod:        demo-web-pod           <-> ${CYAN}demo-web-pod-clone${NC}\n"
    printf "  - Pod:        demo-multi-container   <-> ${CYAN}demo-multi-container-clone${NC}\n"
    printf "  - Pod:        demo-scheduling-pod    <-> ${CYAN}demo-scheduling-pod-clone${NC}\n"
    printf "  - Pod:        demo-storage-pod       <-> ${CYAN}demo-storage-pod-clone${NC}\n"
    printf "  - Deployment: demo-order-api         <-> ${CYAN}demo-order-api-clone${NC}\n"
    printf "  - Cross-kind: demo-order-api (dep)   <-> ${CYAN}demo-order-api-pod-clone${NC} (pod)\n"
    printf "  - Overrides:  demo-web-pod           <-> ${CYAN}demo-web-pod-override-clone${NC}\n"
    printf "\n"
    log_info "To compare automatically, run:   ./examples/same-as/test-same-as.sh --compare"
    log_info "To clean up all resources, run:  ./examples/same-as/test-same-as.sh --clean"
fi
