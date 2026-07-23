package cmd

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xenos76/kubectl-netdrill/internal/image"
	"github.com/xenos76/kubectl-netdrill/internal/k8s"
	mcpsrv "github.com/xenos76/kubectl-netdrill/internal/mcp"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestMcpCmd_ClientError(t *testing.T) {
	origProvider := k8s.ClientProvider
	origSilenceErrors := rootCmd.SilenceErrors
	origSilenceUsage := rootCmd.SilenceUsage

	defer func() {
		k8s.ClientProvider = origProvider
		rootCmd.SilenceErrors = origSilenceErrors
		rootCmd.SilenceUsage = origSilenceUsage

		resetCmdState()
	}()

	resetCmdState()

	k8s.ClientProvider = func(*genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return nil, nil, errors.New("no kube")
	}

	rootCmd.SetArgs([]string{"mcp", "--owner", "alice", "--resolve-image=false"})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true

	err := rootCmd.ExecuteContext(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting client")
}

func TestMcpCmd_RunWithInjectedServer(t *testing.T) {
	origProvider := k8s.ClientProvider
	origRun := runMCP
	origImage := Image
	origSilenceErrors := rootCmd.SilenceErrors
	origSilenceUsage := rootCmd.SilenceUsage

	defer func() {
		k8s.ClientProvider = origProvider
		runMCP = origRun
		Image = origImage
		rootCmd.SilenceErrors = origSilenceErrors
		rootCmd.SilenceUsage = origSilenceUsage

		resetCmdState()
	}()

	resetCmdState()

	fakeClient := fake.NewSimpleClientset()
	k8s.ClientProvider = func(*genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return fakeClient, &rest.Config{}, nil
	}

	var gotDeps *mcpsrv.Deps

	runMCP = func(_ context.Context, version string, deps *mcpsrv.Deps) error {
		gotDeps = deps

		assert.Equal(t, Version, version)

		return nil
	}

	rootCmd.SetArgs([]string{"mcp", "--owner", "alice", "--resolve-image=false"})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true

	require.NoError(t, rootCmd.ExecuteContext(context.Background()))
	require.NotNil(t, gotDeps)
	assert.Equal(t, "alice", gotDeps.Cfg.Owner)
	assert.Equal(t, "alice", gotDeps.Guard.Owner)
	assert.Equal(t, Image, gotDeps.Cfg.Image)
}

func TestMcpCmd_ResolveImageSuccess(t *testing.T) {
	origProvider := k8s.ClientProvider
	origRun := runMCP
	origResolve := resolveLatestImage
	origImage := Image
	origSilenceErrors := rootCmd.SilenceErrors
	origSilenceUsage := rootCmd.SilenceUsage

	defer func() {
		k8s.ClientProvider = origProvider
		runMCP = origRun
		resolveLatestImage = origResolve
		Image = origImage
		rootCmd.SilenceErrors = origSilenceErrors
		rootCmd.SilenceUsage = origSilenceUsage

		resetCmdState()
	}()

	resetCmdState()

	k8s.ClientProvider = func(*genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return fake.NewSimpleClientset(), &rest.Config{}, nil
	}

	resolveLatestImage = func(_ context.Context, _ string, _ *image.RegistryClient) (string, error) {
		return image.DefaultRepo + ":v3.2.1", nil
	}

	var gotImage string

	runMCP = func(_ context.Context, _ string, deps *mcpsrv.Deps) error {
		gotImage = deps.Cfg.Image

		return nil
	}

	Image = image.DefaultRepo + ":latest"
	rootCmd.SetArgs([]string{"mcp", "--owner", "alice", "--resolve-image=true", "--image", Image})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true

	require.NoError(t, rootCmd.ExecuteContext(context.Background()))
	assert.Equal(t, image.DefaultRepo+":v3.2.1", gotImage)
}

func TestMcpCmd_ResolveImageWarn(t *testing.T) {
	origProvider := k8s.ClientProvider
	origRun := runMCP
	origResolve := resolveLatestImage
	origImage := Image
	origSilenceErrors := rootCmd.SilenceErrors
	origSilenceUsage := rootCmd.SilenceUsage

	defer func() {
		k8s.ClientProvider = origProvider
		runMCP = origRun
		resolveLatestImage = origResolve
		Image = origImage
		rootCmd.SilenceErrors = origSilenceErrors
		rootCmd.SilenceUsage = origSilenceUsage

		resetCmdState()
	}()

	resetCmdState()

	k8s.ClientProvider = func(*genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return fake.NewSimpleClientset(), &rest.Config{}, nil
	}

	resolveLatestImage = func(_ context.Context, img string, _ *image.RegistryClient) (string, error) {
		return img, errors.New("registry down")
	}

	var gotImage string

	runMCP = func(_ context.Context, _ string, deps *mcpsrv.Deps) error {
		gotImage = deps.Cfg.Image

		return nil
	}

	Image = image.DefaultRepo + ":latest"
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"mcp", "--owner", "alice", "--resolve-image=true", "--image", Image})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true

	require.NoError(t, rootCmd.ExecuteContext(context.Background()))
	assert.Equal(t, Image, gotImage)
}
