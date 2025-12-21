package mirror

import (
	"context"
	"errors"
	"fmt"

	"github.com/boring-registry/boring-registry/pkg/core"
	o11y "github.com/boring-registry/boring-registry/pkg/observability"

	"github.com/go-kit/kit/endpoint"
	"github.com/prometheus/client_golang/prometheus"
)

type listProviderVersionsRequest struct {
	Hostname  string `json:"hostname,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
}

// EmptyObject exists to return an `{}` JSON object to match the protocol spec
type EmptyObject struct{}

// ListProviderVersionsResponse holds the response that is passed to the endpoint
type ListProviderVersionsResponse struct {
	Versions map[string]EmptyObject `json:"versions"`

	// embedded struct to determine if the response was composed of providers from the mirror
	mirrorSource
}

func listProviderVersionsEndpoint(svc Service, metrics *o11y.MirrorMetrics) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req, ok := request.(listProviderVersionsRequest)
		if !ok {
			return nil, fmt.Errorf("type assertion failed for listProviderVersionsRequest")
		}

		metrics.ListProviderVersions.With(prometheus.Labels{
			o11y.HostnameLabel:  req.Hostname,
			o11y.NamespaceLabel: req.Namespace,
			o11y.NameLabel:      req.Name,
		}).Inc()

		provider := &core.Provider{
			Hostname:  req.Hostname,
			Namespace: req.Namespace,
			Name:      req.Name,
		}

		if provider.Hostname == "" || provider.Namespace == "" || provider.Name == "" {
			return nil, core.ErrVarMissing
		}

		return svc.ListProviderVersions(ctx, provider)
	}
}

type listProviderInstallationRequest struct {
	Hostname  string `json:"hostname,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
	Version   string `json:"version,omitempty"`
}

type ListProviderInstallationResponse struct {
	Archives map[string]Archive `json:"archives"`

	// embedded struct to determine if the response was composed of providers from the mirror
	mirrorSource
}

type Archive struct {
	Url    string   `json:"url"`
	Hashes []string `json:"hashes,omitempty"`
}

func listProviderInstallationEndpoint(svc Service, metrics *o11y.MirrorMetrics) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req, ok := request.(listProviderInstallationRequest)
		if !ok {
			return nil, fmt.Errorf("type assertion failed for listProviderInstallationRequest")
		}

		metrics.ListProviderInstallation.With(prometheus.Labels{
			o11y.HostnameLabel:  req.Hostname,
			o11y.NamespaceLabel: req.Namespace,
			o11y.NameLabel:      req.Name,
			o11y.VersionLabel:   req.Version,
		}).Inc()

		provider := &core.Provider{
			Hostname:  req.Hostname,
			Namespace: req.Namespace,
			Name:      req.Name,
			Version:   req.Version,
		}

		if provider.Hostname == "" || provider.Namespace == "" || provider.Name == "" || provider.Version == "" {
			return nil, errors.New("invalid parameters")
		}

		return svc.ListProviderInstallation(ctx, provider)
	}
}

type retrieveProviderArchiveRequest struct {
	Hostname     string `json:"hostname,omitempty"`
	Namespace    string `json:"namespace,omitempty"`
	Name         string `json:"name,omitempty"`
	Version      string `json:"version,omitempty"`
	OS           string `json:"os,omitempty"`
	Architecture string `json:"architecture,omitempty"`
}

type retrieveProviderArchiveResponse struct {
	location string

	// embedded struct to determine if the response was composed of providers from the mirror
	mirrorSource
}

func retrieveProviderArchiveEndpoint(svc Service, metrics *o11y.MirrorMetrics) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req, ok := request.(retrieveProviderArchiveRequest)
		if !ok {
			return nil, fmt.Errorf("type assertion failed for retrieveProviderArchiveRequest")
		}

		metrics.RetrieveProviderArchive.With(prometheus.Labels{
			o11y.HostnameLabel:  req.Hostname,
			o11y.NamespaceLabel: req.Namespace,
			o11y.NameLabel:      req.Name,
			o11y.VersionLabel:   req.Version,
			o11y.OsLabel:        req.OS,
			o11y.ArchLabel:      req.Architecture,
		}).Inc()

		provider := &core.Provider{
			Hostname:  req.Hostname,
			Namespace: req.Namespace,
			Name:      req.Name,
			Version:   req.Version,
			OS:        req.OS,
			Arch:      req.Architecture,
		}

		if provider.Hostname == "" || provider.Namespace == "" || provider.Name == "" || provider.Version == "" || provider.OS == "" || provider.Arch == "" {
			return nil, core.ErrVarMissing
		}

		return svc.RetrieveProviderArchive(ctx, provider)
	}
}

type retrieveSha256SumsRequest struct {
	Hostname  string `json:"hostname,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
	Version   string `json:"version,omitempty"`
}

type retrieveSha256SumsResponse struct {
	content  []byte
	fileName string
	location string // URL for redirect if upstream

	// embedded struct to determine if the response was composed of providers from the mirror
	mirrorSource
}

func retrieveSha256SumsEndpoint(svc Service, metrics *o11y.MirrorMetrics) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req, ok := request.(retrieveSha256SumsRequest)
		if !ok {
			return nil, fmt.Errorf("type assertion failed for retrieveSha256SumsRequest")
		}

		metrics.RetrieveSha256Sums.With(prometheus.Labels{
			o11y.HostnameLabel:  req.Hostname,
			o11y.NamespaceLabel: req.Namespace,
			o11y.NameLabel:      req.Name,
			o11y.VersionLabel:   req.Version,
		}).Inc()

		provider := &core.Provider{
			Hostname:  req.Hostname,
			Namespace: req.Namespace,
			Name:      req.Name,
			Version:   req.Version,
		}

		if provider.Hostname == "" || provider.Namespace == "" || provider.Name == "" || provider.Version == "" {
			return nil, core.ErrVarMissing
		}

		return svc.RetrieveSha256Sums(ctx, provider)
	}
}

type retrieveSha256SumsSignatureRequest struct {
	Hostname  string `json:"hostname,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
	Version   string `json:"version,omitempty"`
}

type retrieveSha256SumsSignatureResponse struct {
	content  []byte
	fileName string
	location string // URL for redirect if upstream

	// embedded struct to determine if the response was composed of providers from the mirror
	mirrorSource
}

func retrieveSha256SumsSignatureEndpoint(svc Service, metrics *o11y.MirrorMetrics) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req, ok := request.(retrieveSha256SumsSignatureRequest)
		if !ok {
			return nil, fmt.Errorf("type assertion failed for retrieveSha256SumsSignatureRequest")
		}

		metrics.RetrieveSha256SumsSignature.With(prometheus.Labels{
			o11y.HostnameLabel:  req.Hostname,
			o11y.NamespaceLabel: req.Namespace,
			o11y.NameLabel:      req.Name,
			o11y.VersionLabel:   req.Version,
		}).Inc()

		provider := &core.Provider{
			Hostname:  req.Hostname,
			Namespace: req.Namespace,
			Name:      req.Name,
			Version:   req.Version,
		}

		if provider.Hostname == "" || provider.Namespace == "" || provider.Name == "" || provider.Version == "" {
			return nil, core.ErrVarMissing
		}

		return svc.RetrieveSha256SumsSignature(ctx, provider)
	}
}
