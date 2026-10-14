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

// completeNodeSelector provides shell autocompletion for the --node-selector flag.
// It handles empty input proposals, label key filtering, and value completion after '='.
func completeNodeSelector(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	nodeLabels, err := FetchNodeLabels()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	allPairs, err := nodeLabels.ToCompletions()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var suggestions []string

	currentToken := toComplete

	var prefix string

	if idx := strings.LastIndex(toComplete, ","); idx != -1 {
		prefix = toComplete[:idx+1]
		currentToken = toComplete[idx+1:]
	}

	if strings.Contains(currentToken, "=") {
		// User typed "key=partialVal". The shell splits on '=', so the active token
		// being replaced by readline/zsh is just the part after '='.
		keyPrefix := currentToken[:strings.Index(currentToken, "=")+1] // e.g. "gitea="

		for _, pair := range allPairs {
			if strings.HasPrefix(pair, currentToken) {
				// Strip the "key=" prefix so only the value completes
				valOnly := strings.TrimPrefix(pair, keyPrefix)
				suggestions = append(suggestions, valOnly)
			}
		}

		slices.Sort(suggestions)
		// NoSpace allows appending comma or continuation without a trailing space
		return suggestions, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
	}

	// User is still typing the key (or empty token). Propose full "key=" or "key=value"
	for _, pair := range allPairs {
		if strings.HasPrefix(pair, currentToken) {
			suggestions = append(suggestions, prefix+pair)
		}
	}

	slices.Sort(suggestions)

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
