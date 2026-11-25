package cmd

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xenos76/kubectl-netdrill/internal/image"
	"github.com/xenos76/kubectl-netdrill/internal/k8s"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func saveAndRestoreVerifyState(t *testing.T) {
	t.Helper()

	origVerifier := verifyImageFn
	origVerify := VerifyImage
	origKey := CosignKey
	origID := CosignIdentity
	origIssuer := CosignOIDCIssuer

	t.Cleanup(func() {
		verifyImageFn = origVerifier
		VerifyImage = origVerify
		CosignKey = origKey
		CosignIdentity = origID
		CosignOIDCIssuer = origIssuer
	})
}

func TestVerifyAndPinImage_BypassAndMissing(t *testing.T) {
	saveAndRestoreVerifyState(t)

	t.Run("disabled with --verify=false", func(t *testing.T) {
		VerifyImage = false
		verifyImageFn = func(_ context.Context, _ image.VerifyOptions) (*image.VerifyResult, error) {
			t.Fatal("verifyImageFn should not be called when VerifyImage is false")
			return nil, nil
		}

		var errBuf bytes.Buffer

		got, err := verifyAndPinImage(context.Background(), "ghcr.io/xenos76/netdrill:v1.0.0", &errBuf)
		require.NoError(t, err)
		assert.Equal(t, "ghcr.io/xenos76/netdrill:v1.0.0", got)
		assert.Empty(t, errBuf.String())
	})

	t.Run("missing signature warns and skips", func(t *testing.T) {
		VerifyImage = true
		verifyImageFn = func(_ context.Context, _ image.VerifyOptions) (*image.VerifyResult, error) {
			return &image.VerifyResult{
				Status: image.VerifyStatusSkippedMissing,
				Digest: "sha256:abcd",
			}, nil
		}

		var errBuf bytes.Buffer

		got, err := verifyAndPinImage(context.Background(), "busybox:latest", &errBuf)
		require.NoError(t, err)
		assert.Equal(t, "busybox:latest", got)
		assert.Contains(t, errBuf.String(), "Warning: busybox:latest has no cosign signatures; skipping verification")
	})
}

func TestVerifyAndPinImage_VerifiedAndInvalid(t *testing.T) {
	saveAndRestoreVerifyState(t)

	t.Run("valid signature pins resolved digest", func(t *testing.T) {
		VerifyImage = true
		CosignIdentity = "^https://github.com/xenOs76/netdrill/"
		CosignOIDCIssuer = image.DefaultGitHubOIDCIssuer

		verifyImageFn = func(_ context.Context, opts image.VerifyOptions) (*image.VerifyResult, error) {
			assert.Equal(t, "ghcr.io/xenos76/netdrill:v1.0.0", opts.Image)
			assert.Equal(t, "^https://github.com/xenOs76/netdrill/", opts.Identity)
			assert.Equal(t, image.DefaultGitHubOIDCIssuer, opts.OIDCIssuer)

			return &image.VerifyResult{
				Status:      image.VerifyStatusVerified,
				ResolvedRef: "ghcr.io/xenos76/netdrill@sha256:1234",
				Digest:      "sha256:1234",
			}, nil
		}

		var errBuf bytes.Buffer

		got, err := verifyAndPinImage(context.Background(), "ghcr.io/xenos76/netdrill:v1.0.0", &errBuf)
		require.NoError(t, err)
		assert.Equal(t, "ghcr.io/xenos76/netdrill@sha256:1234", got)
		assert.Contains(t, errBuf.String(), "Verified cosign signature")
	})

	t.Run("invalid signature fails with hint", func(t *testing.T) {
		VerifyImage = true
		verifyImageFn = func(_ context.Context, _ image.VerifyOptions) (*image.VerifyResult, error) {
			return nil, errors.New("signature mismatch")
		}

		var errBuf bytes.Buffer

		_, err := verifyAndPinImage(context.Background(), "ghcr.io/xenos76/netdrill:v1.0.0", &errBuf)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "use --verify=false to skip")
		assert.Contains(t, err.Error(), "signature mismatch")
	})
}

func TestPodCommand_VerifyIntegration(t *testing.T) {
	resetCmdState()

	origProvider := k8s.ClientProvider
	origVerifier := verifyImageFn

	t.Cleanup(func() {
		k8s.ClientProvider = origProvider
		verifyImageFn = origVerifier

		resetCmdState()
	})

	k8s.ClientProvider = func(_ *genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return fake.NewSimpleClientset(), &rest.Config{}, nil
	}

	verifyImageFn = func(_ context.Context, _ image.VerifyOptions) (*image.VerifyResult, error) {
		return nil, errors.New("invalid signature")
	}

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"pod", "test-pod"})

	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cosign verification failed")
}

func TestPodCommand_DryRunVerifiesAndPinsDigest(t *testing.T) {
	resetCmdState()

	origProvider := k8s.ClientProvider
	origVerifier := verifyImageFn

	t.Cleanup(func() {
		k8s.ClientProvider = origProvider
		verifyImageFn = origVerifier

		resetCmdState()
	})

	k8s.ClientProvider = func(_ *genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		t.Fatal("ClientProvider should not be called for client-side dry-run")
		return nil, nil, nil
	}

	verifyImageFn = func(_ context.Context, _ image.VerifyOptions) (*image.VerifyResult, error) {
		return &image.VerifyResult{
			Status:      image.VerifyStatusVerified,
			ResolvedRef: "ghcr.io/xenos76/netdrill@sha256:deadbeef",
			Digest:      "sha256:deadbeef",
		}, nil
	}

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"pod", "test-pod", "--dry-run=client", "-o", "yaml"})

	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, outBuf.String(), "ghcr.io/xenos76/netdrill@sha256:deadbeef")
	assert.Contains(t, errBuf.String(), "Verified cosign signature")
}

func TestCosignFlagCompletions(t *testing.T) {
	ids, dir := completeCosignIdentity(nil, nil, "")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, dir)
	assert.NotEmpty(t, ids)

	issuers, dir := completeCosignOIDCIssuer(nil, nil, "https://token")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, dir)
	assert.Equal(t, []string{"https://token.actions.githubusercontent.com"}, issuers)
}
