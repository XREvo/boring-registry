package module

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/boring-registry/boring-registry/pkg/core"
)

// ModuleCopier defines the interface for copying modules to mirror storage.
type ModuleCopier interface {
	// Copy copies the module to the mirror storage asynchronously
	Copy(module core.Module, hostname, downloadURL string)
}

// moduleCopier implements ModuleCopier and ensures that requested modules are replicated to the internal storage asynchronously
type moduleCopier struct {
	// done is used to signal termination to potentially multiple goroutines at once
	done chan struct{}

	storage MirrorStorage
	client  *http.Client
	logger  *slog.Logger
}

// Copy should be started in a separate goroutine
func (c *moduleCopier) Copy(module core.Module, hostname, downloadURL string) {
	begin := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// A goroutine that terminates all pending downloads in case the application is shutting down
	go func() {
		select {
		case <-c.done:
			cancel()
		case <-ctx.Done():
			// No-op as the copy process either succeeded and the deferred cancel() function was called
			// or the operation timed out. In both cases, we just want to terminate the goroutine
		}
	}()

	// Download the module from upstream
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		c.logger.Error("failed to create module download request",
			logModuleKeyValues(module, hostname),
			slog.String("url", downloadURL),
			slog.String("err", err.Error()))
		return
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Error("failed to download module",
			logModuleKeyValues(module, hostname),
			slog.String("url", downloadURL),
			slog.String("err", err.Error()))
		return
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Warn("failed to close response body",
				logModuleKeyValues(module, hostname),
				slog.String("err", err.Error()))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		c.logger.Error("module download returned non-200 status",
			logModuleKeyValues(module, hostname),
			slog.String("url", downloadURL),
			slog.Int("status", resp.StatusCode))
		return
	}

	// Upload to mirror storage
	_, err = c.storage.UploadMirroredModule(ctx, hostname, module.Namespace, module.Name, module.Provider, module.Version, resp.Body)
	if err != nil {
		c.logger.Error("failed to upload module to mirror",
			logModuleKeyValues(module, hostname),
			slog.String("err", err.Error()))
		return
	}

	c.logger.Info("successfully copied module",
		logModuleKeyValues(module, hostname),
		slog.String("took", time.Since(begin).String()))
}

func (c *moduleCopier) shutdown(ctx context.Context) {
	<-ctx.Done()
	close(c.done)
}

// NewModuleCopier creates a new module copier.
func NewModuleCopier(ctx context.Context, storage MirrorStorage) ModuleCopier {
	logger := slog.Default().With(slog.String("component", "module-copier"))
	m := &moduleCopier{
		done:    make(chan struct{}),
		logger:  logger,
		storage: storage,
		client: &http.Client{
			// This is also the timeout for reading the response body
			Timeout: 4 * time.Minute,
		},
	}
	go m.shutdown(ctx)
	return m
}

func logModuleKeyValues(module core.Module, hostname string) slog.Attr {
	return slog.Group("module",
		slog.String("hostname", hostname),
		slog.String("namespace", module.Namespace),
		slog.String("name", module.Name),
		slog.String("provider", module.Provider),
		slog.String("version", module.Version),
	)
}
