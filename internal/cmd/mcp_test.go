package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xenos76/kubectl-netdrill/internal/k8s"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestMcpCmdRegistered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"mcp"})
	require.NoError(t, err)
	assert.Equal(t, "mcp", cmd.Name())
}

func TestMcpCmdFlags(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"mcp"})
	require.NoError(t, err)

	require.NotNil(t, cmd.Flags().Lookup("owner"))
	require.NotNil(t, cmd.Flags().Lookup("exec-timeout"))
	require.NotNil(t, cmd.Flags().Lookup("max-output-bytes"))
	require.NotNil(t, cmd.Flags().Lookup("insecure-allow-any-pod"))
	require.NotNil(t, cmd.Flags().Lookup("mirror-exec-to-logs"))
}

func TestMcpCmd_MissingOwner(t *testing.T) {
	origProvider := k8s.ClientProvider
	origOwner := mcpOwner
	origSilenceErrors := rootCmd.SilenceErrors
	origSilenceUsage := rootCmd.SilenceUsage

	defer func() {
		k8s.ClientProvider = origProvider
		mcpOwner = origOwner
		rootCmd.SilenceErrors = origSilenceErrors
		rootCmd.SilenceUsage = origSilenceUsage

		resetCmdState()
	}()

	resetCmdState()

	k8s.ClientProvider = func(_ *genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return fake.NewSimpleClientset(), &rest.Config{}, nil
	}

	t.Setenv("USER", "")

	mcpOwner = ""

	_, _, err := rootCmd.Find([]string{"mcp"})
	require.NoError(t, err)

	rootCmd.SetArgs([]string{"mcp"})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true

	err = rootCmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "owner")
}
