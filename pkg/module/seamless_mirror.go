package module

import (
	"context"
	"errors"
	"log/slog"

	"github.com/boring-registry/boring-registry/pkg/core"
)

// SeamlessMirrorConfig contains configuration for the seamless mirror module service.
type SeamlessMirrorConfig struct {
	UpstreamHostname string
}

// seamlessMirrorModuleService wraps a module.Service with fallback to mirrored modules
// and upstream registry. It implements the following lookup order:
// 1. Local storage (internal modules)
// 2. Mirror storage (cached modules from upstream)
// 3. Upstream registry (fetch from remote and cache asynchronously)
type seamlessMirrorModuleService struct {
	local         Service
	mirrorStorage MirrorStorage
	upstream      UpstreamModule
	copier        ModuleCopier
	config        SeamlessMirrorConfig
	logger        *slog.Logger
}

// NewSeamlessMirrorModuleService creates a new seamless mirror service that wraps the local module
// service with fallback to mirrored storage and upstream registry.
func NewSeamlessMirrorModuleService(
	local Service,
	mirrorStorage MirrorStorage,
	upstream UpstreamModule,
	copier ModuleCopier,
	config SeamlessMirrorConfig,
) Service {
	return &seamlessMirrorModuleService{
		local:         local,
		mirrorStorage: mirrorStorage,
		upstream:      upstream,
		copier:        copier,
		config:        config,
		logger:        slog.Default().With(slog.String("component", "seamless-mirror-module")),
	}
}

func (s *seamlessMirrorModuleService) GetModule(ctx context.Context, namespace, name, provider, version string) (core.Module, error) {
	// 1. Try local storage first
	m, err := s.local.GetModule(ctx, namespace, name, provider, version)
	if err == nil {
		s.logger.Debug("module found in local storage",
			slog.String("namespace", namespace),
			slog.String("name", name),
			slog.String("provider", provider),
			slog.String("version", version),
		)
		return m, nil
	}

	// Check if it's a "not found" error - if not, return the error
	if !errors.Is(err, ErrModuleNotFound) {
		var objectNotFoundErr *core.ObjectNotFoundError
		if !errors.As(err, &objectNotFoundErr) {
			return core.Module{}, err
		}
	}

	// 2. Try mirror storage
	m, err = s.mirrorStorage.GetMirroredModule(ctx, s.config.UpstreamHostname, namespace, name, provider, version)
	if err == nil {
		s.logger.Debug("module found in mirror storage",
			slog.String("hostname", s.config.UpstreamHostname),
			slog.String("namespace", namespace),
			slog.String("name", name),
			slog.String("provider", provider),
			slog.String("version", version),
		)
		return m, nil
	}

	// Check if it's a "not found" error - if not, return the error
	if !errors.Is(err, ErrModuleNotFound) {
		var objectNotFoundErr *core.ObjectNotFoundError
		if !errors.As(err, &objectNotFoundErr) {
			return core.Module{}, err
		}
	}

	// 3. Fetch download URL from upstream
	s.logger.Debug("fetching module from upstream",
		slog.String("hostname", s.config.UpstreamHostname),
		slog.String("namespace", namespace),
		slog.String("name", name),
		slog.String("provider", provider),
		slog.String("version", version),
	)

	downloadURL, err := s.upstream.GetModuleDownloadURL(ctx, s.config.UpstreamHostname, namespace, name, provider, version)
	if err != nil {
		return core.Module{}, err
	}

	// Create module with the download URL from upstream
	upstreamModule := core.Module{
		Namespace:   namespace,
		Name:        name,
		Provider:    provider,
		Version:     version,
		DownloadURL: downloadURL,
	}

	// Copy to mirror storage asynchronously
	go s.copier.Copy(upstreamModule, s.config.UpstreamHostname, downloadURL)

	return upstreamModule, nil
}

func (s *seamlessMirrorModuleService) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]core.Module, error) {
	// 1. Try local storage first
	versions, err := s.local.ListModuleVersions(ctx, namespace, name, provider)
	if err == nil && len(versions) > 0 {
		s.logger.Debug("module versions found in local storage",
			slog.String("namespace", namespace),
			slog.String("name", name),
			slog.String("provider", provider),
			slog.Int("count", len(versions)),
		)
		return versions, nil
	}

	// Check if it's a "not found" error - if not, return the error
	if err != nil && !errors.Is(err, ErrModuleNotFound) {
		var objectNotFoundErr *core.ObjectNotFoundError
		if !errors.As(err, &objectNotFoundErr) {
			return nil, err
		}
	}

	// 2. Try mirror storage
	mirroredModules, err := s.mirrorStorage.ListMirroredModuleVersions(ctx, s.config.UpstreamHostname, namespace, name, provider)
	if err == nil && len(mirroredModules) > 0 {
		s.logger.Debug("module versions found in mirror storage",
			slog.String("hostname", s.config.UpstreamHostname),
			slog.String("namespace", namespace),
			slog.String("name", name),
			slog.String("provider", provider),
			slog.Int("count", len(mirroredModules)),
		)
		return mirroredModules, nil
	}

	// Check if it's a "not found" error - if not, return the error
	if err != nil && !errors.Is(err, ErrModuleNotFound) {
		var objectNotFoundErr *core.ObjectNotFoundError
		if !errors.As(err, &objectNotFoundErr) {
			return nil, err
		}
	}

	// 3. Fetch from upstream
	s.logger.Debug("fetching module versions from upstream",
		slog.String("hostname", s.config.UpstreamHostname),
		slog.String("namespace", namespace),
		slog.String("name", name),
		slog.String("provider", provider),
	)

	return s.upstream.ListModuleVersions(ctx, s.config.UpstreamHostname, namespace, name, provider)
}
