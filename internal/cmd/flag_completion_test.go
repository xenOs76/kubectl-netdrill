package cmd

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/xenos76/kubectl-netdrill/internal/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

// TestNodeLabels tests receiver safety, deduplication, and completion output of NodeLabels.
func TestNodeLabels(t *testing.T) {
	t.Run("nil receiver safety", func(t *testing.T) {
		var nl *NodeLabels

		require.Error(t, nl.AddNodeLabelsToValues(map[string]string{"k": "v"}))
		_, err := nl.ToCompletions()
		require.Error(t, err)
	})

	t.Run("uninitialized receiver safety", func(t *testing.T) {
		nl := &NodeLabels{}

		require.Error(t, nl.AddNodeLabelsToValues(map[string]string{"k": "v"}))
		_, err := nl.ToCompletions()
		require.Error(t, err)
	})

	t.Run("deduplication and formatting", func(t *testing.T) {
		nl := NewNodeLabels()

		err := nl.AddNodeLabelsToValues(map[string]string{
			"os":   "linux",
			"arch": "arm64",
		})
		require.NoError(t, err)

		// Second node with identical os but different arch
		err = nl.AddNodeLabelsToValues(map[string]string{
			"os":   "linux",
			"arch": "amd64",
		})
		require.NoError(t, err)

		comps, err := nl.ToCompletions()
		require.NoError(t, err)

		require.ElementsMatch(t, []string{
			"os=linux",
			"arch=arm64",
			"arch=amd64",
		}, comps)
	})
}

// newTestNodeClient creates a fake kubernetes clientset seeded with two test nodes.
func newTestNodeClient() *fake.Clientset {
	return fake.NewSimpleClientset(
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-1",
				Labels: map[string]string{
					"kubernetes.io/os": "linux",
					"topology/zone":    "zone-a",
				},
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-2",
				Labels: map[string]string{
					"kubernetes.io/os": "linux",
					"topology/zone":    "zone-b",
				},
			},
		},
	)
}

// TestCompleteNodeSelector verifies autocompletion suggestions under different input states,
// including empty input, prefix matching, post-equals value completion, and comma-separated chaining.
func TestCompleteNodeSelector(t *testing.T) {
	origProvider := k8s.ClientProvider
	defer func() {
		k8s.ClientProvider = origProvider
	}()

	fakeClient := newTestNodeClient()
	k8s.ClientProvider = func(_ *genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return fakeClient, &rest.Config{}, nil
	}

	tests := []struct {
		name            string
		toComplete      string
		wantDirect      cobra.ShellCompDirective
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:         "empty token proposes all pairs",
			toComplete:   "",
			wantDirect:   cobra.ShellCompDirectiveDefault,
			wantContains: []string{"kubernetes.io/os=linux", "topology/zone=zone-a", "topology/zone=zone-b"},
		},
		{
			name:         "filter by key prefix",
			toComplete:   "topo",
			wantDirect:   cobra.ShellCompDirectiveDefault,
			wantContains: []string{"topology/zone=zone-a", "topology/zone=zone-b"},
		},
		{
			name:         "value completion after equals",
			toComplete:   "topology/zone=",
			wantDirect:   cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp,
			wantContains: []string{"topology/zone=zone-a", "topology/zone=zone-b"},
		},
		{
			name:         "partial value completion",
			toComplete:   "topology/zone=zone-a",
			wantDirect:   cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp,
			wantContains: []string{"topology/zone=zone-a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, dir := completeNodeSelector(nil, nil, tt.toComplete)
			require.Equal(t, tt.wantDirect, dir)

			for _, item := range tt.wantContains {
				require.Contains(t, got, item)
			}
		})
	}
}

// TestCompleteNodeSelector_Comma verifies that autocompletion stops immediately on a comma
// to prevent shell Readline recursion.
func TestCompleteNodeSelector_Comma(t *testing.T) {
	tests := []struct {
		name       string
		toComplete string
	}{
		{
			name:       "trailing comma",
			toComplete: "kubernetes.io/os=linux,",
		},
		{
			name:       "comma with prefix",
			toComplete: "kubernetes.io/os=linux,top",
		},
		{
			name:       "second comma",
			toComplete: "kubernetes.io/os=linux,topology/zone=zone-a,",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, dir := completeNodeSelector(nil, nil, tt.toComplete)
			require.Equal(t, cobra.ShellCompDirectiveNoFileComp, dir)
			require.Empty(t, got)
		})
	}
}

// TestCompleteNodeSelector_RepeatedFlag verifies that already-configured node-selector keys
// on the command are excluded from autocompletion proposals.
func TestCompleteNodeSelector_RepeatedFlag(t *testing.T) {
	origProvider := k8s.ClientProvider
	defer func() {
		k8s.ClientProvider = origProvider
	}()

	fakeClient := newTestNodeClient()
	k8s.ClientProvider = func(_ *genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return fakeClient, &rest.Config{}, nil
	}

	var nsMap map[string]string

	cmd := &cobra.Command{}
	cmd.Flags().StringToStringVar(&nsMap, "node-selector", map[string]string{"kubernetes.io/os": "linux"}, "")

	got, dir := completeNodeSelector(cmd, nil, "")
	require.Equal(t, cobra.ShellCompDirectiveDefault, dir)
	require.NotContains(t, got, "kubernetes.io/os=linux")
	require.Contains(t, got, "topology/zone=zone-a")
	require.Contains(t, got, "topology/zone=zone-b")
}

// TestCompleteNodeSelector_Error verifies that completeNodeSelector gracefully returns
// ShellCompDirectiveError when the Kubernetes client cannot be initialized.
func TestCompleteNodeSelector_Error(t *testing.T) {
	origProvider := k8s.ClientProvider
	defer func() {
		k8s.ClientProvider = origProvider
	}()

	k8s.ClientProvider = func(_ *genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return nil, nil, errors.New("client connection failed")
	}

	got, dir := completeNodeSelector(nil, nil, "")
	require.Nil(t, got)
	require.Equal(t, cobra.ShellCompDirectiveError, dir)
}

// newTestSAClient creates a fake kubernetes clientset seeded with service accounts across namespaces.
func newTestSAClient() *fake.Clientset {
	return fake.NewSimpleClientset(
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default",
				Namespace: "default",
			},
		},
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "build-robot",
				Namespace: "default",
			},
		},
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "builder-agent",
				Namespace: "default",
			},
		},
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "custom-sa",
				Namespace: "custom-ns",
			},
		},
	)
}

// TestCompleteServiceAccount verifies autocompletion suggestions for --service-account.
func TestCompleteServiceAccount(t *testing.T) {
	origProvider := k8s.ClientProvider
	defer func() {
		k8s.ClientProvider = origProvider
	}()

	fakeClient := newTestSAClient()
	k8s.ClientProvider = func(_ *genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return fakeClient, &rest.Config{}, nil
	}

	tests := []struct {
		name         string
		toComplete   string
		wantDirect   cobra.ShellCompDirective
		wantContains []string
	}{
		{
			name:         "empty token proposes all service accounts in default namespace",
			toComplete:   "",
			wantDirect:   cobra.ShellCompDirectiveNoFileComp,
			wantContains: []string{"build-robot", "builder-agent", "default"},
		},
		{
			name:         "filter by prefix",
			toComplete:   "build",
			wantDirect:   cobra.ShellCompDirectiveNoFileComp,
			wantContains: []string{"build-robot", "builder-agent"},
		},
		{
			name:         "non-matching prefix yields empty",
			toComplete:   "nonexistent",
			wantDirect:   cobra.ShellCompDirectiveNoFileComp,
			wantContains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, dir := completeServiceAccount(nil, nil, tt.toComplete)
			require.Equal(t, tt.wantDirect, dir)

			for _, item := range tt.wantContains {
				require.Contains(t, got, item)
			}
		})
	}

	t.Run("completes in custom namespace", func(t *testing.T) {
		origFlags := ConfigFlags
		defer func() {
			ConfigFlags = origFlags
		}()

		ConfigFlags = genericclioptions.NewConfigFlags(true)
		ns := "custom-ns"
		ConfigFlags.Namespace = &ns

		got, dir := completeServiceAccount(nil, nil, "")
		require.Equal(t, cobra.ShellCompDirectiveNoFileComp, dir)
		require.Equal(t, []string{"custom-sa"}, got)
	})
}

// TestCompleteServiceAccount_Error verifies that completeServiceAccount returns ShellCompDirectiveError on failure.
func TestCompleteServiceAccount_Error(t *testing.T) {
	origProvider := k8s.ClientProvider
	defer func() {
		k8s.ClientProvider = origProvider
	}()

	k8s.ClientProvider = func(_ *genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return nil, nil, errors.New("client connection failed")
	}

	got, dir := completeServiceAccount(nil, nil, "")
	require.Nil(t, got)
	require.Equal(t, cobra.ShellCompDirectiveError, dir)
}

// TestGetTargetNamespace verifies fallback and configured namespace retrieval.
func TestGetTargetNamespace(t *testing.T) {
	origFlags := ConfigFlags
	defer func() {
		ConfigFlags = origFlags
	}()

	ConfigFlags = nil

	require.Equal(t, "default", getTargetNamespace())

	ConfigFlags = genericclioptions.NewConfigFlags(true)
	custom := "custom-ns"
	ConfigFlags.Namespace = &custom

	require.Equal(t, "custom-ns", getTargetNamespace())
}

// newTestNamespaceClient creates a fake kubernetes clientset seeded with test namespaces.
func newTestNamespaceClient() *fake.Clientset {
	return fake.NewSimpleClientset(
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "default",
			},
		},
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kube-system",
			},
		},
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kube-public",
			},
		},
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "production",
			},
		},
	)
}

// TestCompleteNamespace verifies autocompletion suggestions for --namespace / -n.
func TestCompleteNamespace(t *testing.T) {
	origProvider := k8s.ClientProvider
	defer func() {
		k8s.ClientProvider = origProvider
	}()

	fakeClient := newTestNamespaceClient()
	k8s.ClientProvider = func(_ *genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return fakeClient, &rest.Config{}, nil
	}

	tests := []struct {
		name         string
		toComplete   string
		wantDirect   cobra.ShellCompDirective
		wantContains []string
	}{
		{
			name:         "empty token proposes all cluster namespaces",
			toComplete:   "",
			wantDirect:   cobra.ShellCompDirectiveNoFileComp,
			wantContains: []string{"default", "kube-public", "kube-system", "production"},
		},
		{
			name:         "filter by prefix",
			toComplete:   "kube-",
			wantDirect:   cobra.ShellCompDirectiveNoFileComp,
			wantContains: []string{"kube-public", "kube-system"},
		},
		{
			name:         "non-matching prefix yields empty",
			toComplete:   "nonexistent",
			wantDirect:   cobra.ShellCompDirectiveNoFileComp,
			wantContains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, dir := completeNamespace(nil, nil, tt.toComplete)
			require.Equal(t, tt.wantDirect, dir)

			for _, item := range tt.wantContains {
				require.Contains(t, got, item)
			}
		})
	}
}

// TestCompleteNamespace_Error verifies that completeNamespace returns ShellCompDirectiveError on failure.
func TestCompleteNamespace_Error(t *testing.T) {
	origProvider := k8s.ClientProvider
	defer func() {
		k8s.ClientProvider = origProvider
	}()

	k8s.ClientProvider = func(_ *genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return nil, nil, errors.New("client connection failed")
	}

	got, dir := completeNamespace(nil, nil, "")
	require.Nil(t, got)
	require.Equal(t, cobra.ShellCompDirectiveError, dir)
}

// TestCompleteReplicas verifies autocompletion suggestions for --replicas.
func TestCompleteReplicas(t *testing.T) {
	got, dir := completeReplicas(nil, nil, "")
	require.Equal(t, cobra.ShellCompDirectiveNoFileComp, dir)
	require.Contains(t, got, "1")
	require.Contains(t, got, "3")

	filtered, _ := completeReplicas(nil, nil, "1")
	require.Contains(t, filtered, "1")
	require.Contains(t, filtered, "10")
	require.NotContains(t, filtered, "2")
}

// TestCompleteCPU verifies autocompletion suggestions for CPU flags.
func TestCompleteCPU(t *testing.T) {
	got, dir := completeCPU(nil, nil, "")
	require.Equal(t, cobra.ShellCompDirectiveNoFileComp, dir)
	require.Contains(t, got, "100m")
	require.Contains(t, got, "500m")
	require.Contains(t, got, "1")

	filtered, _ := completeCPU(nil, nil, "2")
	require.Contains(t, filtered, "200m")
	require.Contains(t, filtered, "2")
	require.NotContains(t, filtered, "100m")
}

// TestCompleteMemory verifies autocompletion suggestions for memory flags.
func TestCompleteMemory(t *testing.T) {
	got, dir := completeMemory(nil, nil, "")
	require.Equal(t, cobra.ShellCompDirectiveNoFileComp, dir)
	require.Contains(t, got, "128Mi")
	require.Contains(t, got, "1Gi")

	filtered, _ := completeMemory(nil, nil, "2")
	require.Contains(t, filtered, "256Mi")
	require.Contains(t, filtered, "2Gi")
	require.NotContains(t, filtered, "128Mi")
}

// TestCompletePort verifies autocompletion suggestions for --port.
func TestCompletePort(t *testing.T) {
	got, dir := completePort(nil, nil, "")
	require.Equal(t, cobra.ShellCompDirectiveNoFileComp, dir)
	require.Contains(t, got, "80")
	require.Contains(t, got, "443")

	filtered, _ := completePort(nil, nil, "80")
	require.Contains(t, filtered, "80")
	require.Contains(t, filtered, "8080")
	require.NotContains(t, filtered, "443")
}

// TestCompleteLabels verifies autocompletion suggestions for --labels.
func TestCompleteLabels(t *testing.T) {
	t.Run("empty token proposes sample labels", func(t *testing.T) {
		got, dir := completeLabels(nil, nil, "")
		require.Equal(t, cobra.ShellCompDirectiveDefault, dir)
		require.Contains(t, got, "app=netdrill")
		require.Contains(t, got, "env=prod")
	})

	t.Run("value completion after equals uses NoSpace", func(t *testing.T) {
		got, dir := completeLabels(nil, nil, "env=")
		require.Equal(t, cobra.ShellCompDirectiveNoSpace|cobra.ShellCompDirectiveNoFileComp, dir)
		require.Contains(t, got, "env=dev")
		require.Contains(t, got, "env=prod")
		require.Contains(t, got, "env=staging")
		require.NotContains(t, got, "app=netdrill")
	})

	t.Run("stops completion on comma", func(t *testing.T) {
		got, dir := completeLabels(nil, nil, "app=netdrill,")
		require.Equal(t, cobra.ShellCompDirectiveNoFileComp, dir)
		require.Empty(t, got)
	})

	t.Run("excludes keys already set on command", func(t *testing.T) {
		var lblMap map[string]string

		cmd := &cobra.Command{}
		cmd.Flags().StringToStringVar(&lblMap, "labels", map[string]string{"app": "netdrill"}, "")

		got, dir := completeLabels(cmd, nil, "")
		require.Equal(t, cobra.ShellCompDirectiveDefault, dir)
		require.NotContains(t, got, "app=netdrill")
		require.Contains(t, got, "env=prod")
	})
}
