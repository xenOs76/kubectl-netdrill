package mcp

import (
	"errors"
	"fmt"
	"slices"

	"github.com/xenos76/kubectl-netdrill/internal/netdrill"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// ErrForbidden indicates the guard rejected an operation.
var ErrForbidden = errors.New("forbidden: pod not authorized for this MCP session")

// Guard enforces MCP access policy on netdrill-managed resources.
type Guard struct {
	Owner       string
	AllowAnyPod bool
	MaxOutput   int64
}

// AuthorizePod checks labels on a pod before mutating operations.
func (g *Guard) AuthorizePod(pod *corev1.Pod, ticketID string) error {
	if g.AllowAnyPod {
		return nil
	}

	if pod.Labels[netdrill.LabelManaged] != netdrill.LabelManagedValue {
		return fmt.Errorf("%w: missing managed label", ErrForbidden)
	}

	podOwner := pod.Labels[netdrill.LabelOwner]
	if g.Owner != "" && podOwner != g.Owner {
		return fmt.Errorf("%w: owner %q does not match session owner %q", ErrForbidden, podOwner, g.Owner)
	}

	podTicket := pod.Labels[netdrill.LabelTicket]
	if podTicket != "" && podTicket != ticketID {
		return fmt.Errorf("%w: ticket %q required, got %q", ErrForbidden, podTicket, ticketID)
	}

	return nil
}

// AuthorizeDeployment checks deployment labels before delete.
func (g *Guard) AuthorizeDeployment(dep *appsv1.Deployment, ticketID string) error {
	if g.AllowAnyPod {
		return nil
	}

	if dep.Labels[netdrill.LabelManaged] != netdrill.LabelManagedValue {
		return fmt.Errorf("%w: deployment not managed by kubectl-netdrill", ErrForbidden)
	}

	depOwner := dep.Labels[netdrill.LabelOwner]
	if g.Owner != "" && depOwner != g.Owner {
		return fmt.Errorf("%w: owner %q does not match session owner %q", ErrForbidden, depOwner, g.Owner)
	}

	depTicket := dep.Labels[netdrill.LabelTicket]
	if depTicket != "" && depTicket != ticketID {
		return fmt.Errorf("%w: ticket %q required, got %q", ErrForbidden, depTicket, ticketID)
	}

	return nil
}

// ValidateContainer ensures the container name is allowed for exec.
func (*Guard) ValidateContainer(name string) error {
	if slices.Contains(netdrill.AllowedExecContainers, name) {
		return nil
	}

	return fmt.Errorf("container %q is not an allowed netdrill container", name)
}

// Truncate limits output size for MCP responses.
func (g *Guard) Truncate(s string) (out string, truncated bool) {
	if g.MaxOutput <= 0 || int64(len(s)) <= g.MaxOutput {
		return s, false
	}

	return s[:g.MaxOutput] + "\n... [truncated]", true
}
