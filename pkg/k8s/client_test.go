package k8s

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func TestGetClient(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
