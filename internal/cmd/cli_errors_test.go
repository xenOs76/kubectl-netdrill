package cmd

import (
	"bytes"
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xenos76/kubectl-netdrill/internal/k8s"
	"github.com/xenos76/kubectl-netdrill/internal/term"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stest "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/remotecommand"
)

func TestRootRunE_Help(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	require.NoError(t, rootCmd.RunE(rootCmd, nil))
	require.Contains(t, buf.String(), "kubectl-netdrill")
}

func TestRunCmd_CreateWaitAttachRawModeErrors(t *testing.T) {
	origProvider := k8s.ClientProvider
	origWait := k8s.WaitForPodReady
	origAttach := k8s.AttachURLGetter
	origSPDY := k8s.SPDYExecutorCreator
	origRaw := term.RawModeSetter
	origMonitor := k8s.MonitorPodStatus

	defer func() {
		k8s.ClientProvider = origProvider
		k8s.WaitForPodReady = origWait
		k8s.AttachURLGetter = origAttach
		k8s.SPDYExecutorCreator = origSPDY
		term.RawModeSetter = origRaw
		k8s.MonitorPodStatus = origMonitor

		resetCmdState()
	}()

	resetCmdState()

	k8s.MonitorPodStatus = func(context.Context, kubernetes.Interface, string, string) error { return nil }

	client := fake.NewSimpleClientset()
	client.PrependReactor("create", "pods", func(k8stest.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("create boom")
	})

	k8s.ClientProvider = func(*genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return client, &rest.Config{}, nil
	}

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	rootCmd.SetArgs([]string{"run", "dup-run"})
	require.Error(t, rootCmd.ExecuteContext(context.Background()))

	k8s.ClientProvider = func(*genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return fake.NewSimpleClientset(), &rest.Config{}, nil
	}
	k8s.WaitForPodReady = func(context.Context, kubernetes.Interface, string, string) error {
		return errors.New("wait fail")
	}

	rootCmd.SetArgs([]string{"run", "wait-fail"})
	require.Error(t, rootCmd.ExecuteContext(context.Background()))

	client = fake.NewSimpleClientset()
	client.PrependReactor("delete", "pods", func(k8stest.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("delete boom")
	})

	k8s.ClientProvider = func(*genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return client, &rest.Config{}, nil
	}
	k8s.WaitForPodReady = func(context.Context, kubernetes.Interface, string, string) error { return nil }
	term.RawModeSetter = func() (func(), error) { return nil, errors.New("raw fail") }
	k8s.AttachURLGetter = func(kubernetes.Interface, string, string, string) (*url.URL, error) {
		return nil, errors.New("attach fail")
	}
	k8s.SPDYExecutorCreator = func(*rest.Config, string, *url.URL) (remotecommand.Executor, error) {
		return nil, errors.New("unused")
	}

	rootCmd.SetArgs([]string{"run", "attach-fail"})
	require.Error(t, rootCmd.ExecuteContext(context.Background()))
}

func TestDebugCmd_AddWaitAttachErrors(t *testing.T) {
	origProvider := k8s.ClientProvider
	origWait := k8s.WaitForEphemeralContainerReady
	origAttach := k8s.AttachURLGetter
	origSPDY := k8s.SPDYExecutorCreator
	origRaw := term.RawModeSetter

	defer func() {
		k8s.ClientProvider = origProvider
		k8s.WaitForEphemeralContainerReady = origWait
		k8s.AttachURLGetter = origAttach
		k8s.SPDYExecutorCreator = origSPDY
		term.RawModeSetter = origRaw

		resetCmdState()
	}()

	resetCmdState()

	k8s.ClientProvider = func(*genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return fake.NewSimpleClientset(), &rest.Config{}, nil
	}

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	rootCmd.SetArgs([]string{"debug", "missing"})
	require.Error(t, rootCmd.ExecuteContext(context.Background()))

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "dbg", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
	}
	k8s.ClientProvider = func(*genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return fake.NewSimpleClientset(pod), &rest.Config{}, nil
	}
	k8s.WaitForEphemeralContainerReady = func(context.Context, kubernetes.Interface, string, string, string) error {
		return errors.New("ephemeral wait fail")
	}

	rootCmd.SetArgs([]string{"debug", "dbg"})
	require.Error(t, rootCmd.ExecuteContext(context.Background()))

	k8s.ClientProvider = func(*genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return fake.NewSimpleClientset(pod.DeepCopy()), &rest.Config{}, nil
	}
	k8s.WaitForEphemeralContainerReady = func(context.Context, kubernetes.Interface, string, string, string) error {
		return nil
	}
	term.RawModeSetter = func() (func(), error) { return nil, errors.New("raw fail") }
	k8s.AttachURLGetter = func(kubernetes.Interface, string, string, string) (*url.URL, error) {
		return nil, errors.New("attach fail")
	}

	rootCmd.SetArgs([]string{"debug", "dbg"})
	require.Error(t, rootCmd.ExecuteContext(context.Background()))
}

func TestPodCmd_CreateError(t *testing.T) {
	requireCreateCmdError(t, "pods", "create boom", []string{"pod", "dup-pod"})
}

func TestDeploymentCmd_CreateError(t *testing.T) {
	requireCreateCmdError(t, "deployments", "create dep boom", []string{"deployment", "boom-dep"})
}

func requireCreateCmdError(t *testing.T, resource, msg string, args []string) {
	t.Helper()

	origProvider := k8s.ClientProvider

	defer func() {
		k8s.ClientProvider = origProvider

		resetCmdState()
	}()

	resetCmdState()

	client := fake.NewSimpleClientset()
	client.PrependReactor("create", resource, func(k8stest.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New(msg)
	})

	k8s.ClientProvider = func(*genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return client, &rest.Config{}, nil
	}

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	rootCmd.SetArgs(args)
	require.Error(t, rootCmd.ExecuteContext(context.Background()))
}
