package mcp

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/xenos76/kubectl-netdrill/internal/k8s"
	"github.com/xenos76/kubectl-netdrill/internal/netdrill"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type namespaceInput struct {
	Namespace string `json:"namespace,omitempty" jsonschema:"Kubernetes namespace (defaults to MCP -n flag)"`
	TicketID  string `json:"ticketId,omitempty" jsonschema:"Ticket ID for delete/exec when pod is ticket-tagged"`
}

type podCreateInput struct {
	namespaceInput
	PodName string `json:"podName,omitempty" jsonschema:"Pod name (default netdrill)"`
}

type podNameInput struct {
	namespaceInput
	PodName string `json:"podName" jsonschema:"Pod name"`
}

type podExecInput struct {
	podNameInput
	Command       []string `json:"command" jsonschema:"Command and arguments to run in the container"`
	ContainerName string   `json:"containerName,omitempty" jsonschema:"Container name (default netdrill)"`
}

type runCreateInput struct {
	podCreateInput
	Command []string `json:"command,omitempty" jsonschema:"Optional one-shot command instead of sleep loop"`
}

type deploymentCreateInput struct {
	namespaceInput
	Name string `json:"name,omitempty" jsonschema:"Deployment name (default netdrill)"`
}

type deploymentNameInput struct {
	namespaceInput
	Name string `json:"name" jsonschema:"Deployment name"`
}

type debugAddInput struct {
	podNameInput
	TargetContainer string `json:"targetContainer,omitempty" jsonschema:"Container to share process namespace with"`
}

type listPodsInput struct {
	namespaceInput
}

type podCreateOutput struct {
	Namespace string `json:"namespace"`
	PodName   string `json:"podName"`
	Owner     string `json:"owner"`
	Ticket    string `json:"ticket,omitempty"`
}

type podExecOutput struct {
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	ExitCode  int    `json:"exitCode"`
	Truncated bool   `json:"truncated,omitempty"`
}

type listPodEntry struct {
	Name      string `json:"name"`
	Owner     string `json:"owner"`
	Ticket    string `json:"ticket,omitempty"`
	Phase     string `json:"phase"`
	Namespace string `json:"namespace"`
}

type listPodsOutput struct {
	Pods []listPodEntry `json:"pods"`
}

func registerTools(server *mcp.Server, deps *Deps) {
	registerPodTools(server, deps)
	registerDeploymentTools(server, deps)
}

func registerPodTools(server *mcp.Server, deps *Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "netdrill_pod_create",
		Description: "Create a persistent netdrill troubleshooting pod",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in podCreateInput) (
		*mcp.CallToolResult, podCreateOutput, error,
	) {
		return handlePodCreate(ctx, deps, in)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "netdrill_pod_delete",
		Description: "Delete a netdrill-managed pod owned by this MCP session",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in podNameInput) (*mcp.CallToolResult, any, error) {
		return handlePodDelete(ctx, deps, in)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "netdrill_pod_wait",
		Description: "Wait until a netdrill pod is Running",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in podNameInput) (*mcp.CallToolResult, any, error) {
		return handlePodWait(ctx, deps, in)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "netdrill_pod_exec",
		Description: "Run a command in a netdrill pod and return stdout/stderr",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in podExecInput) (*mcp.CallToolResult, podExecOutput, error) {
		return handlePodExec(ctx, deps, in)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "netdrill_run_create",
		Description: "Create an ephemeral netdrill pod (not deleted automatically)",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in runCreateInput) (
		*mcp.CallToolResult, podCreateOutput, error,
	) {
		return handleRunCreate(ctx, deps, in)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "netdrill_run_cleanup",
		Description: "Delete an ephemeral netdrill pod",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in podNameInput) (*mcp.CallToolResult, any, error) {
		return handlePodDelete(ctx, deps, in)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "netdrill_debug_add",
		Description: "Add a netdrill-debug ephemeral container to an existing pod",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in debugAddInput) (*mcp.CallToolResult, any, error) {
		return handleDebugAdd(ctx, deps, in)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "netdrill_debug_exec",
		Description: "Run a command in the netdrill-debug ephemeral container",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in podExecInput) (*mcp.CallToolResult, podExecOutput, error) {
		in.ContainerName = netdrill.ContainerDebug
		return handlePodExec(ctx, deps, in)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "netdrill_list_managed_pods",
		Description: "List netdrill-managed pods for this MCP session owner",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listPodsInput) (
		*mcp.CallToolResult, listPodsOutput, error,
	) {
		return handleListPods(ctx, deps, in)
	})
}

func registerDeploymentTools(server *mcp.Server, deps *Deps) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "netdrill_deployment_create",
		Description: "Create a netdrill troubleshooting Deployment",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in deploymentCreateInput) (
		*mcp.CallToolResult, podCreateOutput, error,
	) {
		return handleDeploymentCreate(ctx, deps, in)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "netdrill_deployment_delete",
		Description: "Delete a netdrill-managed Deployment owned by this MCP session",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in deploymentNameInput) (*mcp.CallToolResult, any, error) {
		return handleDeploymentDelete(ctx, deps, in)
	})
}

func resolveNamespace(deps *Deps, ns string) string {
	if ns != "" {
		return ns
	}

	return deps.Cfg.DefaultNamespace
}

func defaultPodName(name string) string {
	if name != "" {
		return name
	}

	return "netdrill"
}

func (deps *Deps) getAuthorizedPod(
	ctx context.Context,
	ns, podName, ticketID string,
) (*corev1.Pod, error) {
	pod, err := deps.Client.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting pod %s/%s: %w", ns, podName, err)
	}

	if err := deps.Guard.AuthorizePod(pod, ticketID); err != nil {
		return nil, err
	}

	return pod, nil
}

func handlePodCreate(ctx context.Context, deps *Deps, in podCreateInput) (*mcp.CallToolResult, podCreateOutput, error) {
	ns := resolveNamespace(deps, in.Namespace)
	podName := defaultPodName(in.PodName)

	cfg := netdrill.PodConfig{
		Namespace: ns,
		PodName:   podName,
		Image:     deps.Cfg.Image,
		Owner:     deps.Cfg.Owner,
		Ticket:    in.TicketID,
	}

	opts := k8s.PodOptionsFromConfig(cfg)

	_, err := k8s.CreatePod(ctx, deps.Client, opts)
	if err != nil {
		return nil, podCreateOutput{}, fmt.Errorf("creating pod: %w", err)
	}

	out := podCreateOutput{
		Namespace: ns,
		PodName:   podName,
		Owner:     deps.Cfg.Owner,
		Ticket:    in.TicketID,
	}

	return nil, out, nil
}

func handleRunCreate(ctx context.Context, deps *Deps, in runCreateInput) (*mcp.CallToolResult, podCreateOutput, error) {
	ns := resolveNamespace(deps, in.Namespace)
	podName := defaultPodName(in.PodName)

	cfg := netdrill.PodConfig{
		Namespace: ns,
		PodName:   podName,
		Image:     deps.Cfg.Image,
		Owner:     deps.Cfg.Owner,
		Ticket:    in.TicketID,
	}

	if len(in.Command) > 0 {
		cfg.Command = slices.Clone(in.Command)
	}

	opts := k8s.PodOptionsFromConfig(cfg)

	_, err := k8s.CreatePod(ctx, deps.Client, opts)
	if err != nil {
		return nil, podCreateOutput{}, fmt.Errorf("creating pod: %w", err)
	}

	out := podCreateOutput{
		Namespace: ns,
		PodName:   podName,
		Owner:     deps.Cfg.Owner,
		Ticket:    in.TicketID,
	}

	return nil, out, nil
}

func handlePodDelete(ctx context.Context, deps *Deps, in podNameInput) (*mcp.CallToolResult, any, error) {
	ns := resolveNamespace(deps, in.Namespace)
	podName := in.PodName

	if podName == "" {
		return nil, nil, errors.New("podName is required")
	}

	if _, err := deps.getAuthorizedPod(ctx, ns, podName, in.TicketID); err != nil {
		return nil, nil, err
	}

	if err := k8s.DeletePod(ctx, deps.Client, ns, podName); err != nil {
		return nil, nil, fmt.Errorf("deleting pod: %w", err)
	}

	return nil, map[string]string{"status": "deleted", "pod_name": podName, "namespace": ns}, nil
}

func handlePodWait(ctx context.Context, deps *Deps, in podNameInput) (*mcp.CallToolResult, any, error) {
	ns := resolveNamespace(deps, in.Namespace)
	podName := in.PodName

	if podName == "" {
		return nil, nil, errors.New("podName is required")
	}

	if _, err := deps.getAuthorizedPod(ctx, ns, podName, in.TicketID); err != nil {
		return nil, nil, err
	}

	if err := k8s.WaitForPodReady(ctx, deps.Client, ns, podName); err != nil {
		return nil, nil, fmt.Errorf("waiting for pod: %w", err)
	}

	return nil, map[string]string{"status": "ready", "pod_name": podName, "namespace": ns}, nil
}

func handlePodExec(ctx context.Context, deps *Deps, in podExecInput) (*mcp.CallToolResult, podExecOutput, error) {
	ns := resolveNamespace(deps, in.Namespace)
	podName := in.PodName

	if podName == "" {
		return nil, podExecOutput{}, errors.New("podName is required")
	}

	if len(in.Command) == 0 {
		return nil, podExecOutput{}, errors.New("command is required")
	}

	container := in.ContainerName
	if container == "" {
		container = netdrill.ContainerNetdrill
	}

	if err := deps.Guard.ValidateContainer(container); err != nil {
		return nil, podExecOutput{}, err
	}

	if _, err := deps.getAuthorizedPod(ctx, ns, podName, in.TicketID); err != nil {
		return nil, podExecOutput{}, err
	}

	execCtx, cancel := context.WithTimeoutCause(ctx, deps.Cfg.ExecTimeout, errors.New("exec timed out"))
	defer cancel()

	result, err := k8s.ExecCommand(execCtx, deps.Client, deps.Config, ns, podName, container,
		slices.Clone(in.Command), deps.Guard.MaxOutput)
	if err != nil && result.ExitCode == 0 && result.Stdout == "" && result.Stderr == "" {
		return nil, podExecOutput{}, err
	}

	stdout, truncOut := deps.Guard.Truncate(result.Stdout)
	stderr, truncErr := deps.Guard.Truncate(result.Stderr)

	out := podExecOutput{
		Stdout:    stdout,
		Stderr:    stderr,
		ExitCode:  result.ExitCode,
		Truncated: truncOut || truncErr,
	}

	if err != nil {
		return nil, out, fmt.Errorf("exec failed (exit %d): %w", result.ExitCode, err)
	}

	return nil, out, nil
}

func handleDeploymentCreate(
	ctx context.Context,
	deps *Deps,
	in deploymentCreateInput,
) (*mcp.CallToolResult, podCreateOutput, error) {
	ns := resolveNamespace(deps, in.Namespace)
	name := defaultPodName(in.Name)
	replicas := int32(1)

	cfg := netdrill.DeploymentConfig{
		PodConfig: netdrill.PodConfig{
			Namespace: ns,
			PodName:   name,
			Image:     deps.Cfg.Image,
			Owner:     deps.Cfg.Owner,
			Ticket:    in.TicketID,
		},
		AppLabel: name,
		Replicas: &replicas,
	}

	opts := k8s.DeploymentOptionsFromConfig(cfg)

	_, err := k8s.CreateDeployment(ctx, deps.Client, opts)
	if err != nil {
		return nil, podCreateOutput{}, fmt.Errorf("creating deployment: %w", err)
	}

	out := podCreateOutput{
		Namespace: ns,
		PodName:   name,
		Owner:     deps.Cfg.Owner,
		Ticket:    in.TicketID,
	}

	return nil, out, nil
}

func handleDeploymentDelete(ctx context.Context, deps *Deps, in deploymentNameInput) (*mcp.CallToolResult, any, error) {
	ns := resolveNamespace(deps, in.Namespace)
	name := in.Name

	if name == "" {
		return nil, nil, errors.New("name is required")
	}

	dep, err := deps.Client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("getting deployment: %w", err)
	}

	if err := deps.Guard.AuthorizeDeployment(dep, in.TicketID); err != nil {
		return nil, nil, err
	}

	if err := k8s.DeleteDeployment(ctx, deps.Client, ns, name); err != nil {
		return nil, nil, fmt.Errorf("deleting deployment: %w", err)
	}

	return nil, map[string]string{"status": "deleted", "name": name, "namespace": ns}, nil
}

func handleDebugAdd(ctx context.Context, deps *Deps, in debugAddInput) (*mcp.CallToolResult, any, error) {
	ns := resolveNamespace(deps, in.Namespace)
	podName := in.PodName

	if podName == "" {
		return nil, nil, errors.New("podName is required")
	}

	if _, err := deps.getAuthorizedPod(ctx, ns, podName, in.TicketID); err != nil {
		return nil, nil, err
	}

	opts := k8s.EphemeralOptions{
		Namespace:     ns,
		PodName:       podName,
		ContainerName: netdrill.ContainerDebug,
		Image:         deps.Cfg.Image,
		TargetProcess: in.TargetContainer,
	}

	if err := k8s.AddEphemeralContainer(ctx, deps.Client, opts); err != nil {
		return nil, nil, fmt.Errorf("adding ephemeral container: %w", err)
	}

	if err := k8s.WaitForEphemeralContainerReady(ctx, deps.Client, ns, podName, netdrill.ContainerDebug); err != nil {
		return nil, nil, fmt.Errorf("waiting for ephemeral container: %w", err)
	}

	return nil, map[string]string{
		"status":         "ready",
		"pod_name":       podName,
		"namespace":      ns,
		"container_name": netdrill.ContainerDebug,
	}, nil
}

func handleListPods(ctx context.Context, deps *Deps, in listPodsInput) (*mcp.CallToolResult, listPodsOutput, error) {
	ns := resolveNamespace(deps, in.Namespace)

	labelSet := labels.Set{
		netdrill.LabelManaged: netdrill.LabelManagedValue,
	}

	if deps.Cfg.Owner != "" && !deps.Guard.AllowAnyPod {
		labelSet[netdrill.LabelOwner] = deps.Cfg.Owner
	}

	if in.TicketID != "" {
		labelSet[netdrill.LabelTicket] = in.TicketID
	}

	pods, err := deps.Client.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
		LabelSelector: labelSet.AsSelector().String(),
	})
	if err != nil {
		return nil, listPodsOutput{}, fmt.Errorf("listing pods: %w", err)
	}

	out := listPodsOutput{Pods: make([]listPodEntry, 0, len(pods.Items))}
	for i := range pods.Items {
		p := &pods.Items[i]
		out.Pods = append(out.Pods, listPodEntry{
			Name:      p.Name,
			Owner:     p.Labels[netdrill.LabelOwner],
			Ticket:    p.Labels[netdrill.LabelTicket],
			Phase:     string(p.Status.Phase),
			Namespace: p.Namespace,
		})
	}

	return nil, out, nil
}
