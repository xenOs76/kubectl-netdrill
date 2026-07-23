// Package image resolves container image references against OCI registries.
package image

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/blang/semver/v4"
)

const (
	// DefaultRepo is the netdrill image repository without tag.
	DefaultRepo = "ghcr.io/xenos76/netdrill"

	defaultGHCR = "https://ghcr.io"
)

// RegistryClient fetches tags from an OCI distribution registry (GHCR-compatible).
type RegistryClient struct {
	// BaseURL is the registry API root (for example https://ghcr.io).
	BaseURL string
	// HTTPClient performs registry HTTP requests; defaults to http.DefaultClient when nil.
	HTTPClient *http.Client
}

// DefaultRegistryClient returns a client for the public GitHub Container Registry.
func DefaultRegistryClient() *RegistryClient {
	return &RegistryClient{
		BaseURL: defaultGHCR,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// defaultRegistryClient is overridable in tests when ResolveIfLatest is called with a nil client.
var defaultRegistryClient = DefaultRegistryClient

// SetDefaultRegistryClient overrides the factory used when ResolveIfLatest gets a nil client.
// It returns a restore function that resets the previous factory.
func SetDefaultRegistryClient(fn func() *RegistryClient) (restore func()) {
	prev := defaultRegistryClient

	if fn == nil {
		fn = DefaultRegistryClient
	}

	defaultRegistryClient = fn

	return func() {
		defaultRegistryClient = prev
	}
}

// ParseReference splits an image reference into repository and tag.
// Digest references (containing @) return the input unchanged as repo with empty tag
// and should not be resolved.
func ParseReference(image string) (repo, tag string) {
	if image == "" {
		return "", ""
	}

	if strings.Contains(image, "@") {
		return image, ""
	}

	lastColon := strings.LastIndex(image, ":")
	if lastColon == -1 {
		return image, ""
	}

	// host:port/... — colon before first slash is part of the host, not a tag.
	slash := strings.Index(image, "/")
	if slash != -1 && lastColon < slash {
		return image, ""
	}

	return image[:lastColon], image[lastColon+1:]
}

// NeedsLatestResolution reports whether the reference should be resolved to a semver tag.
func NeedsLatestResolution(image string) bool {
	if image == "" || strings.Contains(image, "@") {
		return false
	}

	_, tag := ParseReference(image)

	return tag == "" || tag == "latest"
}

// PickLatestSemver returns the highest semver tag from tags (e.g. v0.1.2, 0.1.2).
func PickLatestSemver(tags []string) (string, error) {
	var best semver.Version

	var bestRaw string

	found := false

	for _, raw := range tags {
		v, ok := parseSemverTag(raw)
		if !ok {
			continue
		}

		if !found || v.GT(best) {
			best = v
			bestRaw = raw
			found = true
		}
	}

	if !found {
		return "", errors.New("no semver tags found")
	}

	return bestRaw, nil
}

// parseSemverTag parses raw into a semver version, accepting an optional "v" prefix.
func parseSemverTag(raw string) (semver.Version, bool) {
	s := strings.TrimPrefix(raw, "v")

	v, err := semver.Parse(s)
	if err != nil {
		return semver.Version{}, false
	}

	return v, true
}

// ResolveLatestSemverTag queries the registry for tags and returns the highest semver tag name.
func (c *RegistryClient) ResolveLatestSemverTag(ctx context.Context, repo string) (string, error) {
	if c == nil {
		return "", errors.New("registry client is nil")
	}

	repoPath, err := repoToRegistryPath(repo)
	if err != nil {
		return "", err
	}

	token, err := c.fetchToken(ctx, repoPath)
	if err != nil {
		return "", fmt.Errorf("registry token: %w", err)
	}

	tags, err := c.fetchTags(ctx, repoPath, token)
	if err != nil {
		return "", fmt.Errorf("list tags: %w", err)
	}

	return PickLatestSemver(tags)
}

// repoToRegistryPath converts a full image repository reference to the path used in registry API URLs.
func repoToRegistryPath(repo string) (string, error) {
	repo = strings.TrimPrefix(repo, "https://")
	repo = strings.TrimPrefix(repo, "http://")

	const ghcrHost = "ghcr.io/"
	if strings.HasPrefix(repo, ghcrHost) {
		return strings.TrimPrefix(repo, ghcrHost), nil
	}

	// Allow host/path for tests: registry.example.com/ns/name
	if idx := strings.Index(repo, "/"); idx != -1 {
		host := repo[:idx]

		path := repo[idx+1:]
		if host != "" && path != "" {
			return path, nil
		}
	}

	return "", fmt.Errorf("unsupported repository %q", repo)
}

// isGHCRRepo reports whether repo refers to a GitHub Container Registry image.
func isGHCRRepo(repo string) bool {
	r := strings.TrimPrefix(repo, "https://")
	r = strings.TrimPrefix(r, "http://")

	return strings.HasPrefix(r, "ghcr.io/")
}

// fetchToken obtains a bearer token for read access to repoPath.
func (c *RegistryClient) fetchToken(ctx context.Context, repoPath string) (string, error) {
	base := strings.TrimSuffix(c.BaseURL, "/")
	scope := url.QueryEscape("repository:" + repoPath + ":pull")
	tokenURL := base + "/token?service=ghcr.io&scope=" + scope

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.http().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", statusError(resp)
	}

	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}

	if body.Token == "" {
		return "", errors.New("empty registry token")
	}

	return body.Token, nil
}

// fetchTags lists tag names for repoPath using the given bearer token.
func (c *RegistryClient) fetchTags(ctx context.Context, repoPath, token string) ([]string, error) {
	base := strings.TrimSuffix(c.BaseURL, "/")
	tagsURL := base + "/v2/" + repoPath + "/tags/list"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tagsURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, statusError(resp)
	}

	var body struct {
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	return body.Tags, nil
}

// http returns the HTTP client to use for registry requests.
func (c *RegistryClient) http() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}

	return http.DefaultClient
}

// statusError builds an error from a non-success registry HTTP response.
func statusError(resp *http.Response) error {
	const maxBody = 512

	b, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))

	return fmt.Errorf("registry HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
}

// ResolveIfLatest replaces :latest (or untagged) GHCR references with repo:<max-semver-tag>.
// Non-GHCR repositories are returned unchanged. When resolution is not needed or fails,
// the original image is returned with the error (if any).
func ResolveIfLatest(ctx context.Context, image string, client *RegistryClient) (string, error) {
	if !NeedsLatestResolution(image) {
		return image, nil
	}

	repo, _ := ParseReference(image)
	if repo == "" {
		repo = DefaultRepo
	}

	if !isGHCRRepo(repo) {
		return image, nil
	}

	if client == nil {
		client = defaultRegistryClient()
	}

	tag, err := client.ResolveLatestSemverTag(ctx, repo)
	if err != nil {
		return image, err
	}

	return repo + ":" + tag, nil
}
