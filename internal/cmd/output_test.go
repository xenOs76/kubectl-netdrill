package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xenos76/kubectl-netdrill/internal/k8s"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"
)

func TestValidateDryRunOutput(t *testing.T) {
	tests := []struct {
		name       string
		dryRun     string
		output     string
		wantDryRun bool
		wantFormat string
		wantErr    bool
	}{
		{name: "default none and empty output", dryRun: "none", output: "", wantDryRun: false, wantFormat: ""},
		{name: "empty dry-run and empty output", dryRun: "", output: "", wantDryRun: false, wantFormat: ""},
		{name: "client defaults to yaml", dryRun: "client", output: "", wantDryRun: true, wantFormat: "yaml"},
		{name: "client with yaml", dryRun: "client", output: "yaml", wantDryRun: true, wantFormat: "yaml"},
		{name: "client with json", dryRun: "client", output: "json", wantDryRun: true, wantFormat: "json"},
		{name: "invalid dry-run", dryRun: "server", output: "", wantErr: true},
		{name: "output without dry-run", dryRun: "none", output: "yaml", wantErr: true},
		{name: "invalid output with dry-run", dryRun: "client", output: "xml", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isDryRun, format, err := validateDryRunOutput(tt.dryRun, tt.output)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantDryRun, isDryRun)
			assert.Equal(t, tt.wantFormat, format)
		})
	}
}

func withNoClusterClientProvider(t *testing.T) {
	t.Helper()
	resetCmdState()

	origProvider := k8s.ClientProvider

	t.Cleanup(func() {
		k8s.ClientProvider = origProvider

		resetCmdState()
	})

	k8s.ClientProvider = func(_ *genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		t.Fatal("ClientProvider should not be called for pure client-side dry-run")
		return nil, nil, nil
	}
}

func TestDryRun_PodWithoutCluster(t *testing.T) {
	withNoClusterClientProvider(t)

	t.Run("pod dry-run defaults to yaml", func(t *testing.T) {
		resetCmdState()

		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)

		rootCmd.SetOut(stdout)
		rootCmd.SetErr(stderr)
		rootCmd.SetArgs([]string{"pod", "diag-pod", "--host-network", "--port", "8080", "--dry-run"})

		require.NoError(t, rootCmd.ExecuteContext(context.Background()))

		var pod corev1.Pod
		require.NoError(t, yaml.Unmarshal(stdout.Bytes(), &pod))
		assert.Equal(t, "v1", pod.APIVersion)
		assert.Equal(t, "Pod", pod.Kind)
		assert.Equal(t, "diag-pod", pod.Name)
		assert.True(t, pod.Spec.HostNetwork)
		require.Len(t, pod.Spec.Containers, 1)
		assert.Equal(t, int32(8080), pod.Spec.Containers[0].Ports[0].ContainerPort)
	})

	t.Run("pod dry-run=client -o json", func(t *testing.T) {
		resetCmdState()

		stdout := new(bytes.Buffer)

		rootCmd.SetOut(stdout)
		rootCmd.SetErr(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"pod", "json-pod", "--dry-run=client", "-o", "json"})

		require.NoError(t, rootCmd.ExecuteContext(context.Background()))

		var pod corev1.Pod
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &pod))
		assert.Equal(t, "v1", pod.APIVersion)
		assert.Equal(t, "Pod", pod.Kind)
		assert.Equal(t, "json-pod", pod.Name)
	})
}

func TestDryRun_DeploymentAndRunWithoutCluster(t *testing.T) {
	withNoClusterClientProvider(t)

	t.Run("deployment dry-run=client -o yaml", func(t *testing.T) {
		resetCmdState()

		stdout := new(bytes.Buffer)

		rootCmd.SetOut(stdout)
		rootCmd.SetErr(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"deployment", "diag-deploy", "--replicas", "2", "--dry-run=client", "-o", "yaml"})

		require.NoError(t, rootCmd.ExecuteContext(context.Background()))

		var deploy appsv1.Deployment
		require.NoError(t, yaml.Unmarshal(stdout.Bytes(), &deploy))
		assert.Equal(t, "apps/v1", deploy.APIVersion)
		assert.Equal(t, "Deployment", deploy.Kind)
		assert.Equal(t, "diag-deploy", deploy.Name)
		require.NotNil(t, deploy.Spec.Replicas)
		assert.Equal(t, int32(2), *deploy.Spec.Replicas)
	})

	t.Run("run dry-run=client -o yaml", func(t *testing.T) {
		resetCmdState()

		stdout := new(bytes.Buffer)

		rootCmd.SetOut(stdout)
		rootCmd.SetErr(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"run", "ephem-pod", "--host-network", "--dry-run=client", "-o", "yaml"})

		require.NoError(t, rootCmd.ExecuteContext(context.Background()))

		var pod corev1.Pod
		require.NoError(t, yaml.Unmarshal(stdout.Bytes(), &pod))
		assert.Equal(t, "v1", pod.APIVersion)
		assert.Equal(t, "Pod", pod.Kind)
		assert.Equal(t, "ephem-pod", pod.Name)
		assert.True(t, pod.Spec.HostNetwork)
	})
}

func TestDryRun_SameAsAndDebugWithClusterReadOnly(t *testing.T) {
	resetCmdState()

	origProvider := k8s.ClientProvider
	defer func() {
		k8s.ClientProvider = origProvider

		resetCmdState()
	}()

	targetPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "target-sa",
			Containers: []corev1.Container{
				{Name: "app", Image: "nginx:latest"},
			},
		},
	}

	fakeClient := fake.NewSimpleClientset(targetPod)
	k8s.ClientProvider = func(_ *genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return fakeClient, &rest.Config{}, nil
	}

	t.Run("pod --same-as with --dry-run=client -o yaml", func(t *testing.T) {
		resetCmdState()
		fakeClient.ClearActions()

		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)

		rootCmd.SetOut(stdout)
		rootCmd.SetErr(stderr)
		rootCmd.SetArgs([]string{"pod", "cloned-pod", "--same-as", "target-pod", "--dry-run=client", "-o", "yaml"})

		require.NoError(t, rootCmd.ExecuteContext(context.Background()))

		var pod corev1.Pod
		require.NoError(t, yaml.Unmarshal(stdout.Bytes(), &pod))
		assert.Equal(t, "cloned-pod", pod.Name)
		assert.Equal(t, "target-sa", pod.Spec.ServiceAccountName)

		for _, action := range fakeClient.Actions() {
			assert.Equal(t, "get", action.GetVerb(), "dry-run must not perform mutating actions")
		}
	})

	t.Run("debug --dry-run=client -o yaml", func(t *testing.T) {
		resetCmdState()
		fakeClient.ClearActions()

		stdout := new(bytes.Buffer)

		rootCmd.SetOut(stdout)
		rootCmd.SetErr(new(bytes.Buffer))
		rootCmd.SetArgs([]string{"debug", "target-pod", "--target", "app", "--dry-run=client", "-o", "yaml"})

		require.NoError(t, rootCmd.ExecuteContext(context.Background()))

		var pod corev1.Pod
		require.NoError(t, yaml.Unmarshal(stdout.Bytes(), &pod))
		assert.Equal(t, "v1", pod.APIVersion)
		assert.Equal(t, "Pod", pod.Kind)
		require.Len(t, pod.Spec.EphemeralContainers, 1)
		assert.Equal(t, "netdrill-debug", pod.Spec.EphemeralContainers[0].Name)
		assert.Equal(t, "app", pod.Spec.EphemeralContainers[0].TargetContainerName)

		for _, action := range fakeClient.Actions() {
			assert.Equal(t, "get", action.GetVerb(), "debug dry-run must not perform mutating actions")
		}
	})
}

func TestDryRunOutputCompletions(t *testing.T) {
	dryRunSuggestions, directive := completeDryRun(nil, nil, "c")
	assert.Equal(t, []string{"client"}, dryRunSuggestions)
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)

	outputSuggestions, directive := completeOutput(nil, nil, "")
	assert.Equal(t, []string{"yaml", "json"}, outputSuggestions)
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
}
