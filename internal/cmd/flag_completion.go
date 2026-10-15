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
		"app=netdrill",
		"env=dev",
		"env=prod",
		"env=staging",
		"team=sre",
		"tier=tools",
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

// completeLabels provides autocompletion suggestions for the --labels flag.
func completeLabels(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if strings.Contains(toComplete, ",") {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var usedKeys map[string]struct{}

	if cmd != nil {
		if existing, err := cmd.Flags().GetStringToString("labels"); err == nil && len(existing) > 0 {
			usedKeys = make(map[string]struct{}, len(existing))
			for k := range existing {
				usedKeys[k] = struct{}{}
			}
		}
	}

	var suggestions []string

	for _, l := range sampleLabels {
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
