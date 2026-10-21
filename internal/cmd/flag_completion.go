package cmd

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/xenos76/kubectl-netdrill/internal/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeLabels stores unique label values aggregated across cluster nodes.
type NodeLabels struct {
	// values maps each label key to a list of unique values discovered on cluster nodes.
	values map[string][]string
}

// NewNodeLabels initializes an empty NodeLabels struct.
func NewNodeLabels() *NodeLabels {
	return &NodeLabels{
		values: make(map[string][]string),
	}
}

// AddNodeLabelsToValues adds the labels of a single K8s node to NodeLabels, deduplicating values.
func (u *NodeLabels) AddNodeLabelsToValues(labels map[string]string) error {
	if u == nil {
		return errors.New("nil receiver")
	}

	if u.values == nil {
		return errors.New("uninitialized receiver")
	}

	for k, v := range labels {
		if !slices.Contains(u.values[k], v) {
			u.values[k] = append(u.values[k], v)
		}
	}

	return nil
}

// ToCompletions converts the aggregated NodeLabels into a slice of "key=value"
// completion strings for every unique label key and value pair.
func (u *NodeLabels) ToCompletions() ([]string, error) {
	if u == nil {
		return nil, errors.New("nil receiver")
	}

	if u.values == nil {
		return nil, errors.New("uninitialized receiver")
	}

	var completions []string

	for label, valuesList := range u.values {
		for _, value := range valuesList {
			completions = append(completions, label+"="+value)
		}
	}

	return completions, nil
}

// FetchNodeLabels queries cluster nodes using the default client and aggregates
// their labels into a NodeLabels instance, with a 2-second timeout.
func FetchNodeLabels() (*NodeLabels, error) {
	sl := NewNodeLabels()

	client, _, err := k8s.ClientProvider(ConfigFlags)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	nodeList, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, node := range nodeList.Items {
		_ = sl.AddNodeLabelsToValues(node.Labels)
	}

	return sl, nil
}

// extractUsedKeys returns a set of label keys already specified in prior flag occurrences on cmd.
func extractUsedKeys(cmd *cobra.Command) map[string]struct{} {
	used := make(map[string]struct{})

	if cmd != nil {
		if existing, err := cmd.Flags().GetStringToString("node-selector"); err == nil {
			for k := range existing {
				used[k] = struct{}{}
			}
		}
	}

	return used
}

// completeNodeSelector provides shell autocompletion for the --node-selector flag.
// It handles empty input proposals, label key filtering, and value completion after '='.
// To avoid Readline word-break recursive expansion in shells, completion stops after the first comma.
func completeNodeSelector(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if strings.Contains(toComplete, ",") {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	nodeLabels, err := FetchNodeLabels()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	allPairs, err := nodeLabels.ToCompletions()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var suggestions []string

	usedKeys := extractUsedKeys(cmd)

	for _, pair := range allPairs {
		key, _, _ := strings.Cut(pair, "=")
		if _, alreadyUsed := usedKeys[key]; alreadyUsed {
			continue
		}

		if strings.HasPrefix(pair, toComplete) {
			suggestions = append(suggestions, pair)
		}
	}

	slices.Sort(suggestions)

	if strings.Contains(toComplete, "=") {
		return suggestions, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
	}

	return suggestions, cobra.ShellCompDirectiveDefault
}

// getTargetNamespace determines the active namespace for flag completion,
// checking ConfigFlags and falling back to "default" if unresolved.
func getTargetNamespace() string {
	if ConfigFlags != nil {
		if ns, _, err := ConfigFlags.ToRawKubeConfigLoader().Namespace(); err == nil && ns != "" {
			return ns
		}
	}

	return "default"
}

// FetchServiceAccounts queries the Kubernetes cluster for ServiceAccounts in the given namespace,
// with a 2-second timeout.
func FetchServiceAccounts(namespace string) ([]string, error) {
	client, _, err := k8s.ClientProvider(ConfigFlags)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	saList, err := client.CoreV1().ServiceAccounts(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(saList.Items))
	for _, sa := range saList.Items {
		names = append(names, sa.Name)
	}

	return names, nil
}

// completeServiceAccount provides shell autocompletion for the --service-account flag.
func completeServiceAccount(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	namespace := getTargetNamespace()

	serviceAccounts, err := FetchServiceAccounts(namespace)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var suggestions []string

	for _, sa := range serviceAccounts {
		if strings.HasPrefix(sa, toComplete) {
			suggestions = append(suggestions, sa)
		}
	}

	slices.Sort(suggestions)

	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

// FetchNamespaces queries the Kubernetes cluster for all namespaces,
// with a 2-second timeout.
func FetchNamespaces() ([]string, error) {
	client, _, err := k8s.ClientProvider(ConfigFlags)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	nsList, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		names = append(names, ns.Name)
	}

	return names, nil
}

// completeNamespace provides shell autocompletion for the --namespace / -n persistent flag.
func completeNamespace(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	namespaces, err := FetchNamespaces()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var suggestions []string

	for _, ns := range namespaces {
		if strings.HasPrefix(ns, toComplete) {
			suggestions = append(suggestions, ns)
		}
	}

	slices.Sort(suggestions)

	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

// Sample values for static flag autocompletion.
var (
	sampleReplicas = []string{"1", "2", "3", "5", "10"}
	sampleCPUs     = []string{"100m", "200m", "500m", "1", "2", "4"}
	sampleMemories = []string{"64Mi", "128Mi", "256Mi", "512Mi", "1Gi", "2Gi", "4Gi"}
	samplePorts    = []string{"80", "443", "8080", "8443"}
	sampleLabels   = []string{
		"key=value",
		"env=dev",
		"env=prod",
		"team=sre",
	}
	sampleEnv = []string{
		"KEY=value",
		"AWS_REGION=eu-central-1",
		"AWS_ROLE_ARN=arn:aws:iam::111111111111:role/SampleIAMRole",
		"AWS_WEB_IDENTITY_TOKEN_FILE=/run/secrets/kubernetes.io/serviceaccount/token",
	}
)

func filterPrefix(candidates []string, toComplete string) []string {
	var matches []string

	for _, c := range candidates {
		if strings.HasPrefix(c, toComplete) {
			matches = append(matches, c)
		}
	}

	return matches
}

// completeReplicas provides autocompletion suggestions for the --replicas flag.
func completeReplicas(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return filterPrefix(sampleReplicas, toComplete), cobra.ShellCompDirectiveNoFileComp
}

// completeCPU provides autocompletion suggestions for CPU flags with millicores and core units.
func completeCPU(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return filterPrefix(sampleCPUs, toComplete), cobra.ShellCompDirectiveNoFileComp
}

// completeMemory provides autocompletion suggestions for memory flags with Mi and Gi units.
func completeMemory(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return filterPrefix(sampleMemories, toComplete), cobra.ShellCompDirectiveNoFileComp
}

// completePort provides autocompletion suggestions for the --port flag.
func completePort(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return filterPrefix(samplePorts, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func commandUsedKeys(cmd *cobra.Command, flagName string) map[string]struct{} {
	if cmd == nil {
		return nil
	}

	existing, err := cmd.Flags().GetStringToString(flagName)
	if err != nil || len(existing) == 0 {
		return nil
	}

	used := make(map[string]struct{}, len(existing))
	for k := range existing {
		used[k] = struct{}{}
	}

	return used
}

func labelsCandidatesFromCommand(cmd *cobra.Command) []string {
	workload, err := fetchSameAsWorkloadFromCommand(cmd)
	if err != nil || workload == nil {
		return sampleLabels
	}

	filtered := k8s.FilterLabels(workload.Labels)
	if len(filtered) == 0 {
		return sampleLabels
	}

	candidates := make([]string, 0, len(filtered))
	for k, v := range filtered {
		if v != "" {
			candidates = append(candidates, k+"="+v)
		} else {
			candidates = append(candidates, k)
		}
	}

	slices.Sort(candidates)

	return candidates
}

// completeLabels provides autocompletion suggestions for the --labels flag.
// If --same-as is specified, it suggests filtered labels from the target workload;
// otherwise it falls back to sampleLabels.
func completeLabels(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if strings.Contains(toComplete, ",") {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	usedKeys := commandUsedKeys(cmd, "labels")
	candidates := labelsCandidatesFromCommand(cmd)

	var suggestions []string

	for _, l := range candidates {
		key, _, _ := strings.Cut(l, "=")
		if _, used := usedKeys[key]; used {
			continue
		}

		if strings.HasPrefix(l, toComplete) {
			suggestions = append(suggestions, l)
		}
	}

	if strings.Contains(toComplete, "=") {
		return suggestions, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
	}

	return suggestions, cobra.ShellCompDirectiveDefault
}

// completeEnv provides autocompletion suggestions for the --env flag.
func completeEnv(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if strings.Contains(toComplete, ",") {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	usedKeys := commandUsedKeys(cmd, "env")

	var suggestions []string

	for _, e := range sampleEnv {
		key, _, _ := strings.Cut(e, "=")
		if _, used := usedKeys[key]; used {
			continue
		}

		if strings.HasPrefix(e, toComplete) {
			suggestions = append(suggestions, e)
		}
	}

	if strings.Contains(toComplete, "=") {
		return suggestions, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
	}

	return suggestions, cobra.ShellCompDirectiveDefault
}

// FetchPods queries the Kubernetes cluster for all pod names in the given namespace,
// with a 2-second timeout.
func FetchPods(namespace string) ([]string, error) {
	client, _, err := k8s.ClientProvider(ConfigFlags)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	podList, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(podList.Items))
	for _, p := range podList.Items {
		names = append(names, p.Name)
	}

	return names, nil
}

// FetchDeployments queries the Kubernetes cluster for all deployment names in the given namespace,
// with a 2-second timeout.
func FetchDeployments(namespace string) ([]string, error) {
	client, _, err := k8s.ClientProvider(ConfigFlags)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	deployList, err := client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(deployList.Items))
	for _, d := range deployList.Items {
		names = append(names, d.Name)
	}

	return names, nil
}

func isPodKind(k string) bool {
	return k == "pod" || k == "pods" || k == "po"
}

func isDeployKind(k string) bool {
	return k == "deployment" || k == "deployments" || k == "deploy"
}

func completeWorkloadPods(queryNs, prefix, filterPrefix string) ([]string, error) {
	pods, err := FetchPods(queryNs)
	if err != nil {
		return nil, err
	}

	var suggestions []string

	for _, p := range pods {
		if strings.HasPrefix(p, filterPrefix) {
			suggestions = append(suggestions, prefix+p)
		}
	}

	slices.Sort(suggestions)

	return suggestions, nil
}

func completeWorkloadDeployments(queryNs, prefix, filterPrefix string) ([]string, error) {
	deploys, err := FetchDeployments(queryNs)
	if err != nil {
		return nil, err
	}

	var suggestions []string

	for _, d := range deploys {
		if strings.HasPrefix(d, filterPrefix) {
			suggestions = append(suggestions, prefix+d)
		}
	}

	slices.Sort(suggestions)

	return suggestions, nil
}

func completeDefaultPods(targetNs, toComplete string) ([]string, error) {
	pods, err := FetchPods(targetNs)
	if err != nil {
		return nil, err
	}

	var suggestions []string

	for _, p := range pods {
		c := "pod/" + p
		if strings.HasPrefix(c, toComplete) {
			suggestions = append(suggestions, c)
		}

		if strings.HasPrefix(p, toComplete) {
			suggestions = append(suggestions, p)
		}
	}

	return suggestions, nil
}

func completeDefaultDeployments(targetNs, toComplete string) ([]string, error) {
	deploys, err := FetchDeployments(targetNs)
	if err != nil {
		return nil, err
	}

	var suggestions []string

	for _, d := range deploys {
		c := "deployment/" + d
		if strings.HasPrefix(c, toComplete) {
			suggestions = append(suggestions, c)
		}

		if strings.HasPrefix(d, toComplete) {
			suggestions = append(suggestions, d)
		}
	}

	return suggestions, nil
}

func collectNamespaceItems(items []string, prefix, kind, toComplete string) []string {
	var suggestions []string

	for _, item := range items {
		c := prefix + kind + "/" + item
		if strings.HasPrefix(c, toComplete) {
			suggestions = append(suggestions, c)
		}

		bare := prefix + item
		if strings.HasPrefix(bare, toComplete) {
			suggestions = append(suggestions, bare)
		}
	}

	return suggestions
}

func completeNamespacePods(ns, prefix, toComplete string) ([]string, cobra.ShellCompDirective) {
	pods, err := FetchPods(ns)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	suggestions := collectNamespaceItems(pods, prefix, "pod", toComplete)

	if strings.HasPrefix(toComplete, prefix+"deploy") {
		deploys, err := FetchDeployments(ns)
		if err == nil {
			for _, d := range deploys {
				c := prefix + "deployment/" + d
				if strings.HasPrefix(c, toComplete) {
					suggestions = append(suggestions, c)
				}
			}
		}
	}

	slices.Sort(suggestions)

	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func completeNamespaceDeployments(ns, prefix, toComplete string) ([]string, cobra.ShellCompDirective) {
	deploys, err := FetchDeployments(ns)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	suggestions := collectNamespaceItems(deploys, prefix, "deployment", toComplete)

	if strings.HasPrefix(toComplete, prefix+"pod") || strings.HasPrefix(toComplete, prefix+"po") {
		pods, err := FetchPods(ns)
		if err == nil {
			for _, p := range pods {
				c := prefix + "pod/" + p
				if strings.HasPrefix(c, toComplete) {
					suggestions = append(suggestions, c)
				}
			}
		}
	}

	slices.Sort(suggestions)

	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func completeNamespaceWorkloads(ns, prefix, defaultKind, toComplete string) ([]string, cobra.ShellCompDirective) {
	if defaultKind == "pod" {
		return completeNamespacePods(ns, prefix, toComplete)
	}

	return completeNamespaceDeployments(ns, prefix, toComplete)
}

func completeDefaultWorkloads(targetNs, defaultKind, toComplete string) ([]string, cobra.ShellCompDirective) {
	if defaultKind == "pod" {
		suggestions, err := completeDefaultPods(targetNs, toComplete)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		if strings.HasPrefix(toComplete, "deploy") {
			extra, err := completeWorkloadDeployments(
				targetNs, "deployment/", strings.TrimPrefix(toComplete, "deployment/"),
			)
			if err == nil {
				suggestions = append(suggestions, extra...)
			}
		}

		slices.Sort(suggestions)

		return suggestions, cobra.ShellCompDirectiveNoFileComp
	}

	suggestions, err := completeDefaultDeployments(targetNs, toComplete)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	if strings.HasPrefix(toComplete, "pod") || strings.HasPrefix(toComplete, "po") {
		extra, err := completeWorkloadPods(
			targetNs, "pod/", strings.TrimPrefix(toComplete, "pod/"),
		)
		if err == nil {
			suggestions = append(suggestions, extra...)
		}
	}

	slices.Sort(suggestions)

	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func completeSameAsTwoParts(
	first, filter, targetNs, defaultKind, toComplete string,
) ([]string, cobra.ShellCompDirective) {
	if isPodKind(first) {
		s, err := completeWorkloadPods(targetNs, first+"/", filter)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		return s, cobra.ShellCompDirectiveNoFileComp
	}

	if isDeployKind(first) {
		s, err := completeWorkloadDeployments(targetNs, first+"/", filter)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		return s, cobra.ShellCompDirectiveNoFileComp
	}

	return completeNamespaceWorkloads(first, first+"/", defaultKind, toComplete)
}

func completeSameAsThreeParts(ns, kind, filter string) ([]string, cobra.ShellCompDirective) {
	prefix := ns + "/" + kind + "/"

	if isPodKind(kind) {
		s, err := completeWorkloadPods(ns, prefix, filter)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		return s, cobra.ShellCompDirectiveNoFileComp
	}

	if isDeployKind(kind) {
		s, err := completeWorkloadDeployments(ns, prefix, filter)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		return s, cobra.ShellCompDirectiveNoFileComp
	}

	return nil, cobra.ShellCompDirectiveNoFileComp
}

// completeSameAs provides autocompletion suggestions for the --same-as flag.
func completeSameAs(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	targetNs := getTargetNamespace()
	defaultKind := "pod"

	if cmd != nil && cmd.Name() == "deployment" {
		defaultKind = "deployment"
	}

	parts := strings.Split(toComplete, "/")

	switch len(parts) {
	case 2:
		return completeSameAsTwoParts(parts[0], parts[1], targetNs, defaultKind, toComplete)
	case 3:
		return completeSameAsThreeParts(parts[0], parts[1], parts[2])
	default:
		return completeDefaultWorkloads(targetNs, defaultKind, toComplete)
	}
}

// fetchSameAsWorkloadFromCommand resolves and fetches the target workload specified in --same-as,
// with a 2-second timeout.
func fetchSameAsWorkloadFromCommand(cmd *cobra.Command) (*k8s.SameAsWorkload, error) {
	if cmd == nil {
		return nil, errors.New("command is nil")
	}

	target, err := cmd.Flags().GetString("same-as")
	if err != nil || strings.TrimSpace(target) == "" {
		return nil, errors.New("same-as flag not set")
	}

	defaultKind := "pod"
	if cmd.Name() == "deployment" {
		defaultKind = "deployment"
	}

	targetInfo, err := k8s.ParseSameAsTarget(target, defaultKind, getTargetNamespace())
	if err != nil {
		return nil, err
	}

	client, _, err := k8s.ClientProvider(ConfigFlags)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	return k8s.FetchSameAsWorkload(ctx, client, targetInfo)
}

// completeSameAsContainer provides autocompletion suggestions for the --same-as-container flag.
// It inspects the target workload specified in --same-as and proposes its container names.
func completeSameAsContainer(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if cmd == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	target, err := cmd.Flags().GetString("same-as")
	if err != nil || strings.TrimSpace(target) == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	workload, err := fetchSameAsWorkloadFromCommand(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var suggestions []string

	for _, c := range workload.PodSpec.Containers {
		if strings.HasPrefix(c.Name, toComplete) {
			suggestions = append(suggestions, c.Name)
		}
	}

	slices.Sort(suggestions)

	return suggestions, cobra.ShellCompDirectiveNoFileComp
}
