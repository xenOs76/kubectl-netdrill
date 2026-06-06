package mcp

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterContainerTools(t *testing.T) {
	t.Parallel()

	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "dev"}, nil)
	err := registerContainerTools(server, &Deps{})
	require.NoError(t, err)
	assert.Equal(t, "netdrill-image-tools", ContainerToolCatalogName())
	assert.NotEmpty(t, containerToolsMD)
}

func TestContainerToolsResourceURI(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "netdrill://container-tools", containerToolsResourceURI)
}
