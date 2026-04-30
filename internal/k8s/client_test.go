package k8s

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func TestGetClient(t *testing.T) {
	kubeconfigContent := `apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:6443
    insecure-skip-tls-verify: true
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
kind: Config
users:
- name: test-user
  user:
    token: test-token
`

	tmpDir := t.TempDir()
	kubeconfigPath := filepath.Join(tmpDir, "kubeconfig")
	err := os.WriteFile(kubeconfigPath, []byte(kubeconfigContent), 0o600)
	require.NoError(t, err)

	tests := []struct {
		name        string
		configFlags *genericclioptions.ConfigFlags
		wantErr     bool
	}{
		{
			name:        "valid config flags",
			configFlags: genericclioptions.NewConfigFlags(true),
			wantErr:     false,
		},
		{
			name:        "invalid config flags",
			configFlags: genericclioptions.NewConfigFlags(true),
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := kubeconfigPath
			if tt.wantErr {
				path = "/non/existent/path"
			}

			tt.configFlags.KubeConfig = &path

			client, config, err := GetClient(tt.configFlags)
			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, client)
				require.Nil(t, config)
			} else {
				require.NoError(t, err)
				require.NotNil(t, client)
				require.NotNil(t, config)
			}
		})
	}
}
