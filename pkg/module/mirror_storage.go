package module

import (
	"context"
	"io"

	"github.com/boring-registry/boring-registry/pkg/core"
)

// MirrorStorage represents the storage of mirrored Terraform modules.
// These are modules fetched from upstream registries and cached locally.
type MirrorStorage interface {
	// GetMirroredModule returns the mirrored module or an error if it cannot be found
	GetMirroredModule(ctx context.Context, hostname, namespace, name, provider, version string) (core.Module, error)

	// ListMirroredModuleVersions returns all matching module versions for a given hostname, namespace, name, and provider
	ListMirroredModuleVersions(ctx context.Context, hostname, namespace, name, provider string) ([]core.Module, error)

	// UploadMirroredModule uploads a module to the mirror storage
	UploadMirroredModule(ctx context.Context, hostname, namespace, name, provider, version string, body io.Reader) (core.Module, error)
}
