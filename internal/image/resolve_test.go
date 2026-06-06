package image

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseReference(t *testing.T) {
	t.Parallel()

	tests := []struct {
		image    string
		wantRepo string
		wantTag  string
	}{
		{"ghcr.io/xenos76/netdrill:latest", "ghcr.io/xenos76/netdrill", "latest"},
		{"ghcr.io/xenos76/netdrill", "ghcr.io/xenos76/netdrill", ""},
		{"ghcr.io/xenos76/netdrill:v0.1.2", "ghcr.io/xenos76/netdrill", "v0.1.2"},
		{"ghcr.io/xenos76/netdrill@sha256:abc", "ghcr.io/xenos76/netdrill@sha256:abc", ""},
		{"registry.example.com:5000/foo/bar:v1", "registry.example.com:5000/foo/bar", "v1"},
		{"", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			t.Parallel()

			tt := tt

			repo, tag := ParseReference(tt.image)
			assert.Equal(t, tt.wantRepo, repo)
			assert.Equal(t, tt.wantTag, tag)
		})
	}
}

func TestNeedsLatestResolution(t *testing.T) {
	t.Parallel()

	assert.True(t, NeedsLatestResolution("ghcr.io/xenos76/netdrill:latest"))
	assert.True(t, NeedsLatestResolution("ghcr.io/xenos76/netdrill"))
	assert.False(t, NeedsLatestResolution("ghcr.io/xenos76/netdrill:v0.1.0"))
	assert.False(t, NeedsLatestResolution("ghcr.io/xenos76/netdrill@sha256:deadbeef"))
	assert.False(t, NeedsLatestResolution(""))
}

func TestPickLatestSemver(t *testing.T) {
	t.Parallel()

	tag, err := PickLatestSemver([]string{"latest", "main", "v0.1.1", "v0.1.2", "v0.0.9", "0.1.0"})
	require.NoError(t, err)
	assert.Equal(t, "v0.1.2", tag)
}

func TestPickLatestSemver_noSemver(t *testing.T) {
	t.Parallel()

	_, err := PickLatestSemver([]string{"latest", "main"})
	require.Error(t, err)
}

func TestResolveIfLatest_skipsExplicitTag(t *testing.T) {
	t.Parallel()

	img, err := ResolveIfLatest(t.Context(), "ghcr.io/xenos76/netdrill:v0.1.0", nil)
	require.NoError(t, err)
	assert.Equal(t, "ghcr.io/xenos76/netdrill:v0.1.0", img)
}

func TestResolveLatestSemverTag_mockRegistry(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})
		case "/v2/xenos76/netdrill/tags/list":
			if r.Header.Get("Authorization") != "Bearer test-token" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)

				return
			}

			_ = json.NewEncoder(w).Encode(map[string]any{
				"name": "xenos76/netdrill",
				"tags": []string{"latest", "v0.1.0", "v0.1.2", "nightly"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	client := &RegistryClient{
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
	}

	tag, err := client.ResolveLatestSemverTag(t.Context(), DefaultRepo)
	require.NoError(t, err)
	assert.Equal(t, "v0.1.2", tag)
}

func TestResolveIfLatest_mockRegistry(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "tok"})
		case "/v2/xenos76/netdrill/tags/list":
			_ = json.NewEncoder(w).Encode(map[string]any{"tags": []string{"v0.2.0", "v0.1.9"}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	client := &RegistryClient{BaseURL: srv.URL, HTTPClient: srv.Client()}

	resolved, err := ResolveIfLatest(context.Background(), DefaultRepo+":latest", client)
	require.NoError(t, err)
	assert.Equal(t, DefaultRepo+":v0.2.0", resolved)
}

func TestResolveIfLatest_skipsNonGHCR(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected registry request: %s", r.URL.Path)
	}))
	t.Cleanup(srv.Close)

	client := &RegistryClient{BaseURL: srv.URL, HTTPClient: srv.Client()}

	img := "docker.io/library/nginx:latest"
	resolved, err := ResolveIfLatest(context.Background(), img, client)
	require.NoError(t, err)
	assert.Equal(t, img, resolved)
}

func TestRepoToRegistryPath(t *testing.T) {
	t.Parallel()

	path, err := repoToRegistryPath("ghcr.io/xenos76/netdrill")
	require.NoError(t, err)
	assert.Equal(t, "xenos76/netdrill", path)

	_, err = repoToRegistryPath("invalid")
	require.Error(t, err)
}
