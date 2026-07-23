package cmd

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xenos76/kubectl-netdrill/internal/k8s"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func TestCompletionCmd_UnsupportedShell(t *testing.T) {
	err := completionCmd.RunE(completionCmd, []string{"noshell"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported shell")
}

func TestCompletionCmd(t *testing.T) {
	shells := []string{"bash", "zsh", "fish", "powershell"}

	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			resetCmdState()

			buf := new(bytes.Buffer)
			rootCmd.SetOut(buf)
			rootCmd.SetErr(buf)
			rootCmd.SetArgs([]string{"completion", shell})

			require.NoError(t, rootCmd.ExecuteContext(context.Background()))

			resetCmdState()
		})
	}
}

func TestManCmd(t *testing.T) {
	resetCmdState()

	defer resetCmdState()

	dir := t.TempDir()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"man", "--dest-dir", dir})

	require.NoError(t, rootCmd.ExecuteContext(context.Background()))

	rootCmd.SetArgs([]string{"man", "pod", "--dest-dir", dir})
	require.NoError(t, rootCmd.ExecuteContext(context.Background()))

	rootCmd.SetArgs([]string{"man", "does-not-exist", "--dest-dir", dir})
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true

	err := rootCmd.ExecuteContext(context.Background())
	require.Error(t, err)
}

func TestExecute_Help(t *testing.T) {
	resetCmdState()

	defer resetCmdState()

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"--help"})

	require.NoError(t, Execute())
	assert.Contains(t, buf.String(), "kubectl-netdrill")
}

func TestRootHelp_NoArgs(t *testing.T) {
	resetCmdState()

	defer resetCmdState()

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{})

	require.NoError(t, rootCmd.ExecuteContext(context.Background()))
	assert.Contains(t, buf.String(), "Available Commands")
}

func TestPodCmd_ClientError(t *testing.T) {
	orig := k8s.ClientProvider

	defer func() {
		k8s.ClientProvider = orig

		resetCmdState()
	}()

	resetCmdState()

	k8s.ClientProvider = func(*genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return nil, nil, errors.New("client boom")
	}

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	rootCmd.SetArgs([]string{"pod", "x"})

	err := rootCmd.ExecuteContext(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client boom")
}

func TestDeploymentCmd_ClientError(t *testing.T) {
	orig := k8s.ClientProvider

	defer func() {
		k8s.ClientProvider = orig

		resetCmdState()
	}()

	resetCmdState()

	k8s.ClientProvider = func(*genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return nil, nil, errors.New("client boom")
	}

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	rootCmd.SetArgs([]string{"deployment", "x"})

	err := rootCmd.ExecuteContext(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client boom")
}

func TestRunCmd_ClientError(t *testing.T) {
	orig := k8s.ClientProvider

	defer func() {
		k8s.ClientProvider = orig

		resetCmdState()
	}()

	resetCmdState()

	k8s.ClientProvider = func(*genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return nil, nil, errors.New("client boom")
	}

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	rootCmd.SetArgs([]string{"run", "x"})

	err := rootCmd.ExecuteContext(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client boom")
}

func TestDebugCmd_ClientError(t *testing.T) {
	orig := k8s.ClientProvider

	defer func() {
		k8s.ClientProvider = orig

		resetCmdState()
	}()

	resetCmdState()

	k8s.ClientProvider = func(*genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return nil, nil, errors.New("client boom")
	}

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	rootCmd.SetArgs([]string{"debug", "x"})

	err := rootCmd.ExecuteContext(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client boom")
}
