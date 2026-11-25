package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"
)

func TestSecureFlag_PodAndDeploymentDryRun(t *testing.T) {
	withNoClusterClientProvider(t)

	t.Run("pod --secure --dry-run=client", func(t *testing.T) {
		resetCmdState()

		var stdout, stderr bytes.Buffer

		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stderr)
		rootCmd.SetArgs([]string{"pod", "sec-pod", "--secure", "--dry-run=client", "-o", "yaml"})

		require.NoError(t, rootCmd.Execute())

		var pod corev1.Pod
		require.NoError(t, yaml.Unmarshal(stdout.Bytes(), &pod))
		assert.Equal(t, ptr.To(false), pod.Spec.AutomountServiceAccountToken)
		require.NotNil(t, pod.Spec.SecurityContext)
		assert.Equal(t, ptr.To(true), pod.Spec.SecurityContext.RunAsNonRoot)
		require.Len(t, pod.Spec.Containers, 1)
		assert.Equal(t, ptr.To(true), pod.Spec.Containers[0].SecurityContext.ReadOnlyRootFilesystem)
		assert.Equal(t, ptr.To(false), pod.Spec.Containers[0].SecurityContext.AllowPrivilegeEscalation)
	})

	t.Run("deployment --secure --dry-run=client", func(t *testing.T) {
		resetCmdState()

		var stdout, stderr bytes.Buffer

		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stderr)
		rootCmd.SetArgs([]string{"deployment", "sec-deploy", "--secure", "--dry-run=client", "-o", "yaml"})

		require.NoError(t, rootCmd.Execute())

		var deploy appsv1.Deployment
		require.NoError(t, yaml.Unmarshal(stdout.Bytes(), &deploy))
		assert.Equal(t, ptr.To(false), deploy.Spec.Template.Spec.AutomountServiceAccountToken)
		require.Len(t, deploy.Spec.Template.Spec.Containers, 1)
		assert.Equal(t, ptr.To(true), deploy.Spec.Template.Spec.Containers[0].SecurityContext.ReadOnlyRootFilesystem)
	})

	t.Run("pod --secure --host-network fails", func(t *testing.T) {
		resetCmdState()

		var stdout, stderr bytes.Buffer

		rootCmd.SetOut(&stdout)
		rootCmd.SetErr(&stderr)
		rootCmd.SetArgs([]string{"pod", "bad-pod", "--secure", "--host-network", "--dry-run=client"})

		err := rootCmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--host-network")
	})
}
