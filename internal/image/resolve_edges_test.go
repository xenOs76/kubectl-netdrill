package image

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseReference_HostPortNoTag(t *testing.T) {
	t.Parallel()

	repo, tag := ParseReference("registry:5000/foo")
	assert.Equal(t, "registry:5000/foo", repo)
	assert.Empty(t, tag)
}

func TestRepoToRegistryPath_Variants(t *testing.T) {
	t.Parallel()

	path, err := repoToRegistryPath("https://ghcr.io/xenos76/netdrill")
	require.NoError(t, err)
	assert.Equal(t, "xenos76/netdrill", path)

	path, err = repoToRegistryPath("registry.example.com/ns/name")
	require.NoError(t, err)
	assert.Equal(t, "ns/name", path)
}

func TestResolveLatestSemverTag_NilClient(t *testing.T) {
	t.Parallel()

	var c *RegistryClient

	_, err := c.ResolveLatestSemverTag(t.Context(), DefaultRepo)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestFetchToken_HTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)

	client := &RegistryClient{BaseURL: srv.URL, HTTPClient: srv.Client()}
	_, err := client.fetchToken(t.Context(), "xenos76/netdrill")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "registry HTTP 401")
	assert.Contains(t, err.Error(), "nope")
}

func TestFetchToken_EmptyToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": ""})
	}))
	t.Cleanup(srv.Close)

	client := &RegistryClient{BaseURL: srv.URL, HTTPClient: srv.Client()}
	_, err := client.fetchToken(t.Context(), "xenos76/netdrill")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty registry token")
}

func TestFetchToken_BadJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("{"))
	}))
	t.Cleanup(srv.Close)

	client := &RegistryClient{BaseURL: srv.URL, HTTPClient: srv.Client()}
	_, err := client.fetchToken(t.Context(), "xenos76/netdrill")
	require.Error(t, err)
}

func TestFetchTags_HTTPErrorAndBadJSON(t *testing.T) {
	t.Parallel()

	t.Run("http", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "tags fail", http.StatusInternalServerError)
		}))
		t.Cleanup(srv.Close)

		client := &RegistryClient{BaseURL: srv.URL, HTTPClient: srv.Client()}
		_, err := client.fetchTags(t.Context(), "x/y", "tok")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "registry HTTP 500")
	})

	t.Run("json", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("not-json"))
		}))
		t.Cleanup(srv.Close)

		client := &RegistryClient{BaseURL: srv.URL, HTTPClient: srv.Client()}
		_, err := client.fetchTags(t.Context(), "x/y", "tok")
		require.Error(t, err)
	})
}

func TestRegistryClient_http_NilFallsBack(t *testing.T) {
	t.Parallel()

	c := &RegistryClient{}
	assert.Equal(t, http.DefaultClient, c.http())
}

func TestResolveIfLatest_EmptyImageUnchanged(t *testing.T) {
	t.Parallel()

	resolved, err := ResolveIfLatest(context.Background(), "", nil)
	require.NoError(t, err)
	assert.Empty(t, resolved)
}

func TestResolveIfLatest_ResolveErrorReturnsOriginal(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)

	client := &RegistryClient{BaseURL: srv.URL, HTTPClient: srv.Client()}
	img := DefaultRepo + ":latest"
	resolved, err := ResolveIfLatest(context.Background(), img, client)
	require.Error(t, err)
	assert.Equal(t, img, resolved)
}

func TestSetDefaultRegistryClient(t *testing.T) {
	called := false
	restore := SetDefaultRegistryClient(func() *RegistryClient {
		called = true

		return &RegistryClient{BaseURL: "http://example.invalid"}
	})
	c := defaultRegistryClient()

	assert.True(t, called)
	assert.Equal(t, "http://example.invalid", c.BaseURL)
	restore()

	restoreNil := SetDefaultRegistryClient(nil)

	require.NotNil(t, defaultRegistryClient())
	restoreNil()
}

func TestFetchToken_NewRequestError(t *testing.T) {
	t.Parallel()

	client := &RegistryClient{BaseURL: "://bad", HTTPClient: http.DefaultClient}
	_, err := client.fetchToken(t.Context(), "x/y")
	require.Error(t, err)
}

func TestFetchTags_NewRequestError(t *testing.T) {
	t.Parallel()

	client := &RegistryClient{BaseURL: "://bad", HTTPClient: http.DefaultClient}
	_, err := client.fetchTags(t.Context(), "x/y", "tok")
	require.Error(t, err)
}

func TestResolveLatestSemverTag_BadRepo(t *testing.T) {
	t.Parallel()

	client := &RegistryClient{BaseURL: "http://example.invalid", HTTPClient: http.DefaultClient}
	_, err := client.ResolveLatestSemverTag(t.Context(), "invalid")
	require.Error(t, err)
}

func TestDefaultRegistryClient(t *testing.T) {
	t.Parallel()

	c := DefaultRegistryClient()
	require.NotNil(t, c)
	assert.Equal(t, defaultGHCR, c.BaseURL)
	require.NotNil(t, c.HTTPClient)
}

func TestResolveLatestSemverTag_TagsListError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "t"})

			return
		}

		http.Error(w, "tags down", http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)

	client := &RegistryClient{BaseURL: srv.URL, HTTPClient: srv.Client()}
	_, err := client.ResolveLatestSemverTag(t.Context(), DefaultRepo)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list tags")
}

func TestFetchTokenAndTags_DoError(t *testing.T) {
	t.Parallel()

	client := &RegistryClient{
		BaseURL: "http://127.0.0.1:1",
		HTTPClient: &http.Client{
			Timeout: 50 * time.Millisecond,
		},
	}
	_, err := client.fetchToken(t.Context(), "x/y")
	require.Error(t, err)

	_, err = client.fetchTags(t.Context(), "x/y", "tok")
	require.Error(t, err)
}

func TestResolveIfLatest_NilClientUsesDefaultHook(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "t"})
		case "/v2/xenos76/netdrill/tags/list":
			_ = json.NewEncoder(w).Encode(map[string]any{"tags": []string{"v9.9.9"}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	orig := defaultRegistryClient

	defer func() { defaultRegistryClient = orig }()

	defaultRegistryClient = func() *RegistryClient {
		return &RegistryClient{BaseURL: srv.URL, HTTPClient: srv.Client()}
	}

	resolved, err := ResolveIfLatest(context.Background(), DefaultRepo+":latest", nil)
	require.NoError(t, err)
	assert.Equal(t, DefaultRepo+":v9.9.9", resolved)
}
