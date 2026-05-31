// Package netdrill provides shared constants and helpers for kubectl-netdrill resources.
package netdrill

// Label and container name constants for netdrill-managed Kubernetes resources.
const (
	LabelApp          = "app"
	LabelAppValue     = "kubectl-netdrill"
	LabelManaged      = "kubectl-netdrill.io/managed"
	LabelManagedValue = "true"
	LabelOwner        = "kubectl-netdrill.io/owner"
	LabelTicket       = "kubectl-netdrill.io/ticket"
	ContainerNetdrill = "netdrill"
	ContainerDebug    = "netdrill-debug"
)

// AllowedExecContainers lists container names MCP and exec may target.
var AllowedExecContainers = []string{ContainerNetdrill, ContainerDebug}

// PodLabels returns standard labels for a netdrill pod or deployment template.
// owner and ticket are optional; when empty they are omitted.
func PodLabels(owner, ticket string) map[string]string {
	labels := map[string]string{
		LabelApp:     LabelAppValue,
		LabelManaged: LabelManagedValue,
	}
	if owner != "" {
		labels[LabelOwner] = owner
	}

	if ticket != "" {
		labels[LabelTicket] = ticket
	}

	return labels
}
