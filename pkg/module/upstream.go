package module

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/boring-registry/boring-registry/pkg/core"
	"github.com/boring-registry/boring-registry/pkg/discovery"
)

// UpstreamModule defines the interface for fetching module data from an upstream registry.
type UpstreamModule interface {
	ListModuleVersions(ctx context.Context, hostname, namespace, name, provider string) ([]core.Module, error)
	GetModuleDownloadURL(ctx context.Context, hostname, namespace, name, provider, version string) (string, error)
}

// upstreamModuleRegistry implements UpstreamModule using the Terraform Registry HTTP API.
type upstreamModuleRegistry struct {
	client                 *http.Client
	remoteServiceDiscovery discovery.ServiceDiscoveryResolver
}

// moduleVersionsResponse represents the response from the upstream registry's module versions endpoint.
type moduleVersionsResponse struct {
	Modules []moduleVersionsModule `json:"modules"`
}

type moduleVersionsModule struct {
	Versions []moduleVersion `json:"versions"`
}

type moduleVersion struct {
	Version string `json:"version"`
}

func (u *upstreamModuleRegistry) ListModuleVersions(ctx context.Context, hostname, namespace, name, provider string) ([]core.Module, error) {
	discovered, err := u.remoteServiceDiscovery.Resolve(ctx, hostname)
	if err != nil {
		return nil, err
	}

	// Terraform Registry API: GET /v1/modules/{namespace}/{name}/{provider}/versions
	path := fmt.Sprintf("%s%s/%s/%s/versions", discovered.ModulesV1, namespace, name, provider)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL(discovered.URL.Host, path), nil)
	if err != nil {
		return nil, err
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream registry returned status %d for module versions", resp.StatusCode)
	}

	var versionsResp moduleVersionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&versionsResp); err != nil {
		return nil, fmt.Errorf("failed to decode module versions response: %w", err)
	}

	var modules []core.Module
	if len(versionsResp.Modules) > 0 {
		for _, v := range versionsResp.Modules[0].Versions {
			modules = append(modules, core.Module{
				Namespace: namespace,
				Name:      name,
				Provider:  provider,
				Version:   v.Version,
			})
		}
	}

	return modules, nil
}

func (u *upstreamModuleRegistry) GetModuleDownloadURL(ctx context.Context, hostname, namespace, name, provider, version string) (string, error) {
	discovered, err := u.remoteServiceDiscovery.Resolve(ctx, hostname)
	if err != nil {
		return "", err
	}

	// Terraform Registry API: GET /v1/modules/{namespace}/{name}/{provider}/{version}/download
	// This returns a redirect or X-Terraform-Get header with the actual download URL
	path := fmt.Sprintf("%s%s/%s/%s/%s/download", discovered.ModulesV1, namespace, name, provider, version)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL(discovered.URL.Host, path), nil)
	if err != nil {
		return "", err
	}

	// Don't follow redirects, we want to capture the download URL
	client := &http.Client{
		Transport: u.client.Transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// The download URL can be provided via:
	// 1. X-Terraform-Get header (primary)
	// 2. Location header for 302 redirects (fallback)
	if downloadURL := resp.Header.Get("X-Terraform-Get"); downloadURL != "" {
		return downloadURL, nil
	}

	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently {
		if location := resp.Header.Get("Location"); location != "" {
			return location, nil
		}
	}

	if resp.StatusCode == http.StatusNoContent {
		// 204 No Content with X-Terraform-Get header is the standard response
		// If we reach here without a download URL, something is wrong
		return "", fmt.Errorf("upstream registry returned 204 but no X-Terraform-Get header")
	}

	return "", fmt.Errorf("upstream registry returned status %d, expected download URL", resp.StatusCode)
}

// NewUpstreamModuleRegistry creates a new upstream module registry client.
func NewUpstreamModuleRegistry(remoteServiceDiscovery discovery.ServiceDiscoveryResolver) UpstreamModule {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConnsPerHost = 100
	return &upstreamModuleRegistry{
		client: &http.Client{
			Transport: transport,
		},
		remoteServiceDiscovery: remoteServiceDiscovery,
	}
}

func upstreamURL(hostname, path string) string {
	upstreamUrl := url.URL{
		Scheme: "https",
		Host:   hostname,
		Path:   path,
	}
	return upstreamUrl.String()
}
