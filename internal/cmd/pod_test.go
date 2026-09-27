package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xenos76/kubectl-netdrill/internal/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestPodCmd(t *testing.T) {
	resetCmdState()

	origProvider := k8s.ClientProvider

	defer func() {
		k8s.ClientProvider = origProvider

		resetCmdState()
	}()

	fakeClient := fake.NewSimpleClientset()
	k8s.ClientProvider = func(_ *genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
		return fakeClient, &rest.Config{}, nil
	}

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "no args", args: []string{}, wantErr: false},
		{name: "help", args: []string{"--help"}, wantErr: false},
		{name: "success", args: []string{"test-pod"}, wantErr: false},
		{name: "with labels", args: []string{"test-pod-labels", "--labels", "env=prod,team=infra"}, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetCmdState()
			rootCmd.SetArgs(append([]string{"pod"}, tt.args...))

			buf := new(bytes.Buffer)
			rootCmd.SetOut(buf)
			rootCmd.SetErr(buf)
			rootCmd.SilenceUsage = true
			rootCmd.SilenceErrors = true

			err := rootCmd.ExecuteContext(context.Background())
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if tt.name == "with labels" {
				p, err := fakeClient.CoreV1().Pods("default").Get(
					context.Background(),
					"test-pod-labels",
					metav1.GetOptions{},
				)
				require.NoError(t, err)
				require.Equal(t, "prod", p.Labels["env"])
				require.Equal(t, "infra", p.Labels["team"])
			}

			resetCmdState()
		})
	}
}
