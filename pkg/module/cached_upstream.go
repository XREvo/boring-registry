package module

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/boring-registry/boring-registry/pkg/core"
	o11y "github.com/boring-registry/boring-registry/pkg/observability"
	"github.com/maypok86/otter/v2"
	"github.com/prometheus/client_golang/prometheus"
)

// CacheConfig contains the cache configuration for the module upstream.
type CacheConfig struct {
	Enabled   bool
	TTL       time.Duration
	MaxSizeMB int
}

// cacheEntry represents a cache entry.
type cacheEntry struct {
	data      interface{}
	timestamp time.Time
	sizeBytes int
}

// cachedUpstreamModule wraps an UpstreamModule with caching.
type cachedUpstreamModule struct {
	upstream UpstreamModule
	cache    *otter.Cache[string, *cacheEntry]
	config   CacheConfig
	metrics  *o11y.ModuleMetrics
}

// buildModuleVersionsKey builds a cache key for ListModuleVersions.
func buildModuleVersionsKey(hostname, namespace, name, provider string) string {
	return fmt.Sprintf("module-versions:%s/%s/%s/%s", hostname, namespace, name, provider)
}

// buildModuleDownloadKey builds a cache key for GetModuleDownloadURL.
func buildModuleDownloadKey(hostname, namespace, name, provider, version string) string {
	return fmt.Sprintf("module-download:%s/%s/%s/%s/%s", hostname, namespace, name, provider, version)
}

// estimateSize estimates the size in bytes of an object via JSON marshaling.
func estimateModuleSize(data interface{}) (int, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return 0, err
	}
	return len(bytes), nil
}

// ListModuleVersions implements UpstreamModule with caching.
func (c *cachedUpstreamModule) ListModuleVersions(ctx context.Context, hostname, namespace, name, provider string) ([]core.Module, error) {
	key := buildModuleVersionsKey(hostname, namespace, name, provider)

	// Try to get from cache
	if entry, ok := c.cache.GetIfPresent(key); ok {
		if modules, ok := entry.data.([]core.Module); ok {
			if c.metrics != nil {
				c.metrics.ListVersions.With(prometheus.Labels{
					o11y.NamespaceLabel: namespace,
					o11y.NameLabel:      name,
					o11y.ProviderLabel:  provider,
				}).Inc()
			}
			return modules, nil
		}
	}

	// Cache miss - call upstream
	modules, err := c.upstream.ListModuleVersions(ctx, hostname, namespace, name, provider)
	if err != nil {
		return nil, err
	}

	// Estimate cache entry size
	sizeBytes, err := estimateModuleSize(modules)
	if err != nil {
		slog.Warn("failed to estimate the byte size of module versions list. Cache set will be skipped", slog.String("error", err.Error()))
		return modules, nil
	}

	// Store in cache
	entry := &cacheEntry{
		data:      modules,
		timestamp: time.Now(),
		sizeBytes: sizeBytes,
	}
	c.cache.Set(key, entry)

	return modules, nil
}

// GetModuleDownloadURL implements UpstreamModule with caching.
func (c *cachedUpstreamModule) GetModuleDownloadURL(ctx context.Context, hostname, namespace, name, provider, version string) (string, error) {
	key := buildModuleDownloadKey(hostname, namespace, name, provider, version)

	// Try to get from cache
	if entry, ok := c.cache.GetIfPresent(key); ok {
		if downloadURL, ok := entry.data.(string); ok {
			if c.metrics != nil {
				c.metrics.Download.With(prometheus.Labels{
					o11y.NamespaceLabel: namespace,
					o11y.NameLabel:      name,
					o11y.ProviderLabel:  provider,
					o11y.VersionLabel:   version,
				}).Inc()
			}
			return downloadURL, nil
		}
	}

	// Cache miss - call upstream
	downloadURL, err := c.upstream.GetModuleDownloadURL(ctx, hostname, namespace, name, provider, version)
	if err != nil {
		return "", err
	}

	// Store in cache (download URLs are small strings, so we can estimate the size directly)
	entry := &cacheEntry{
		data:      downloadURL,
		timestamp: time.Now(),
		sizeBytes: len(key) + len(downloadURL),
	}
	c.cache.Set(key, entry)

	return downloadURL, nil
}

// NewCachedUpstreamModule creates a new upstream module wrapper with caching.
func NewCachedUpstreamModule(upstream UpstreamModule, config CacheConfig, metrics *o11y.ModuleMetrics) (UpstreamModule, error) {
	// Convert MB to bytes
	maxWeightBytes := config.MaxSizeMB * 1024 * 1024

	// Configure otter cache
	opts := &otter.Options[string, *cacheEntry]{
		MaximumWeight: uint64(maxWeightBytes),
		Weigher: func(key string, value *cacheEntry) uint32 {
			// Weight = size of key + size of value
			return uint32(len(key) + value.sizeBytes)
		},
		ExpiryCalculator: otter.ExpiryWriting[string, *cacheEntry](config.TTL),
		InitialCapacity:  100,
	}

	cache, err := otter.New(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	return &cachedUpstreamModule{
		upstream: upstream,
		cache:    cache,
		config:   config,
		metrics:  metrics,
	}, nil
}
