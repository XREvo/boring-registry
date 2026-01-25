package provider

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/boring-registry/boring-registry/pkg/core"
	"github.com/boring-registry/boring-registry/pkg/mirror"
)

// SeamlessMirrorConfig contains configuration for the seamless mirror service.
type SeamlessMirrorConfig struct {
	UpstreamHostname string
}

// seamlessMirrorService wraps a provider.Service with fallback to mirrored providers
// and upstream registry. It implements the following lookup order:
// 1. Local storage (internal providers)
// 2. Mirror storage (cached providers from upstream)
// 3. Upstream registry (fetch from remote and cache asynchronously)
type seamlessMirrorService struct {
	local         Service
	mirrorStorage mirror.Storage
	upstream      mirror.UpstreamProvider
	copier        mirror.Copier
	config        SeamlessMirrorConfig
	logger        *slog.Logger
}

// NewSeamlessMirrorService creates a new seamless mirror service that wraps the local provider
// service with fallback to mirrored storage and upstream registry.
func NewSeamlessMirrorService(
	local Service,
	mirrorStorage mirror.Storage,
	upstream mirror.UpstreamProvider,
	copier mirror.Copier,
	config SeamlessMirrorConfig,
) Service {
	return &seamlessMirrorService{
		local:         local,
		mirrorStorage: mirrorStorage,
		upstream:      upstream,
		copier:        copier,
		config:        config,
		logger:        slog.Default().With(slog.String("component", "seamless-mirror-provider")),
	}
}

func (s *seamlessMirrorService) GetProvider(ctx context.Context, namespace, name, version, os, arch string) (*core.Provider, error) {
	// 1. Try local storage first
	p, err := s.local.GetProvider(ctx, namespace, name, version, os, arch)
	if err == nil {
		s.logger.Debug("provider found in local storage",
			slog.String("namespace", namespace),
			slog.String("name", name),
			slog.String("version", version),
			slog.String("os", os),
			slog.String("arch", arch),
		)
		return p, nil
	}

	// Check if it's a "not found" error - if not, return the error
	var providerErr *core.ProviderError
	var objectNotFoundErr *core.ObjectNotFoundError
	if !errors.As(err, &providerErr) && !errors.As(err, &objectNotFoundErr) {
		return nil, err
	}

	// 2. Try mirror storage
	mirrorProvider := &core.Provider{
		Hostname:  s.config.UpstreamHostname,
		Namespace: namespace,
		Name:      name,
		Version:   version,
		OS:        os,
		Arch:      arch,
	}

	p, err = s.mirrorStorage.GetMirroredProvider(ctx, mirrorProvider)
	if err == nil {
		s.logger.Debug("provider found in mirror storage",
			slog.String("hostname", s.config.UpstreamHostname),
			slog.String("namespace", namespace),
			slog.String("name", name),
			slog.String("version", version),
			slog.String("os", os),
			slog.String("arch", arch),
		)
		return p, nil
	}

	// Check if it's a "not found" error - if not, return the error
	if !errors.As(err, &providerErr) && !errors.As(err, &objectNotFoundErr) {
		return nil, err
	}

	// 3. Fetch from upstream
	s.logger.Debug("fetching provider from upstream",
		slog.String("hostname", s.config.UpstreamHostname),
		slog.String("namespace", namespace),
		slog.String("name", name),
		slog.String("version", version),
		slog.String("os", os),
		slog.String("arch", arch),
	)

	upstreamProvider, err := s.upstream.GetProvider(ctx, mirrorProvider)
	if err != nil {
		return nil, err
	}

	// Copy to mirror storage asynchronously
	go s.copier.Copy(upstreamProvider)

	return upstreamProvider, nil
}

func (s *seamlessMirrorService) ListProviderVersions(ctx context.Context, namespace, name string) (*core.ProviderVersions, error) {
	// 1. Try local storage first
	versions, err := s.local.ListProviderVersions(ctx, namespace, name)
	if err == nil && len(versions.Versions) > 0 {
		s.logger.Debug("provider versions found in local storage",
			slog.String("namespace", namespace),
			slog.String("name", name),
			slog.Int("count", len(versions.Versions)),
		)
		return versions, nil
	}

	// Check if it's a "not found" error - if not, return the error
	var providerErr *core.ProviderError
	var objectNotFoundErr *core.ObjectNotFoundError
	if err != nil && !errors.As(err, &providerErr) && !errors.As(err, &objectNotFoundErr) {
		return nil, err
	}

	// 2. Try mirror storage
	mirrorProvider := &core.Provider{
		Hostname:  s.config.UpstreamHostname,
		Namespace: namespace,
		Name:      name,
	}

	mirroredProviders, err := s.mirrorStorage.ListMirroredProviders(ctx, mirrorProvider)
	if err == nil && len(mirroredProviders) > 0 {
		s.logger.Debug("provider versions found in mirror storage",
			slog.String("hostname", s.config.UpstreamHostname),
			slog.String("namespace", namespace),
			slog.String("name", name),
			slog.Int("count", len(mirroredProviders)),
		)
		return s.mirroredProvidersToVersions(mirroredProviders), nil
	}

	// Check if it's a "not found" error - if not, return the error
	if err != nil && !errors.As(err, &providerErr) && !errors.As(err, &objectNotFoundErr) {
		return nil, err
	}

	// 3. Fetch from upstream
	s.logger.Debug("fetching provider versions from upstream",
		slog.String("hostname", s.config.UpstreamHostname),
		slog.String("namespace", namespace),
		slog.String("name", name),
	)

	return s.upstream.ListProviderVersions(ctx, mirrorProvider)
}

// mirroredProvidersToVersions converts a list of mirrored providers to ProviderVersions format.
func (s *seamlessMirrorService) mirroredProvidersToVersions(providers []*core.Provider) *core.ProviderVersions {
	// Group providers by version and collect platforms
	versionMap := make(map[string]*core.ProviderVersion)

	for _, p := range providers {
		id := fmt.Sprintf("%s/%s/%s", p.Namespace, p.Name, p.Version)
		if _, ok := versionMap[id]; !ok {
			versionMap[id] = &core.ProviderVersion{
				Namespace: p.Namespace,
				Name:      p.Name,
				Version:   p.Version,
				Platforms: []core.Platform{},
			}
		}
		versionMap[id].Platforms = append(versionMap[id].Platforms, core.Platform{OS: p.OS, Arch: p.Arch})
	}

	var versions []core.ProviderVersion
	for _, v := range versionMap {
		versions = append(versions, *v)
	}

	return &core.ProviderVersions{Versions: versions}
}
