package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	applicationadmission "ardents/internal/applicationapi/admission"
	applicationcall "ardents/internal/applicationapi/call"
	applicationcontent "ardents/internal/applicationapi/content"
	applicationprincipal "ardents/internal/applicationapi/principal"
	contentdomain "ardents/internal/content"
	"ardents/internal/daemon"
	"ardents/internal/localapi"
	"ardents/internal/observability"
	"ardents/internal/provision"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := provision.Run(os.Args[2:], os.Stdout); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "ardentsd init: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if err := daemon.Run(newLocalAPIHandler, newApplicationAPIHandler, newOperatorSurface); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "ardentsd: %v\n", err)
		os.Exit(1)
	}
}

func newApplicationAPIHandler(process daemon.Owners, cfg daemon.ApplicationAPIConfig) (string, http.Handler, error) {
	if !cfg.Protected {
		return "", nil, fmt.Errorf("Application Interface requires the protected Principal socket")
	}
	injector, extractor := applicationcall.NewChannel()
	interceptor, err := applicationadmission.NewInterceptor(applicationadmission.Config{
		Access: process.PrincipalAccess, Node: cfg.TargetID,
		FallbackPeer: cfg.PeerBinding, FallbackSource: cfg.Source, Injector: injector,
	})
	if err != nil {
		return "", nil, err
	}
	contentPath, contentHandler, err := applicationcontent.NewHTTPHandler(applicationContentStore{owners: process}, extractor, interceptor)
	if err != nil {
		return contentPath, contentHandler, err
	}
	identityPath, identityHandler, err := applicationprincipal.NewHandler(process.PrincipalAccess, cfg.TargetID, cfg.PeerBinding, cfg.Source)
	if err != nil {
		return "", nil, err
	}
	mux := http.NewServeMux()
	mux.Handle(contentPath, contentHandler)
	mux.Handle(identityPath, identityHandler)
	return contentPath, mux, nil
}

type applicationContentStore struct{ owners daemon.Owners }

func (s applicationContentStore) PublishBlob(call applicationcall.Call, command contentdomain.PublishBlobCommand) (contentdomain.Blob, error) {
	return contentdomain.Blob{}, contentdomain.ErrStoreUnavailable
}

func (s applicationContentStore) GetBlob(call applicationcall.Call, id string) (contentdomain.Blob, bool) {
	return contentdomain.Blob{}, false
}

func (s applicationContentStore) GetBlobPayload(call applicationcall.Call, id string) ([]byte, error) {
	return nil, contentdomain.ErrStoreUnavailable
}

func (s applicationContentStore) FetchBlob(ctx context.Context, call applicationcall.Call, id string) (contentdomain.Blob, error) {
	return contentdomain.Blob{}, contentdomain.ErrStoreUnavailable
}

func newLocalAPIHandler(process daemon.Owners, cfg daemon.LocalAPIConfig) (string, http.Handler, error) {
	runtime := process.Node
	deps := localapi.Dependencies{
		Node: runtime, Discovery: runtime, DiscoveryRecords: process.DiscoveryCommands, Network: runtime,
		Diagnostics: process.Diagnostics, Workload: process.Workloads, Hosting: process.Hosting,
		Content: process.Content, Sources: process.Content, Transfers: process.Transfers,
		Data: process.ContentCommands, DataFetch: runtime, Configuration: runtime, Audit: process.Events,
	}
	if !cfg.Protected {
		return "", nil, fmt.Errorf("Operator Interface requires the protected Principal socket")
	}
	return localapi.NewProtectedHandler(deps, process.PrincipalAccess, cfg.TargetID, cfg.PeerBinding, cfg.Source)
}

func newOperatorSurface(process daemon.Owners, token string) (daemon.OperatorSurface, error) {
	return observability.NewSurface(observability.Dependencies{
		Runtime: process.Node, Diagnostics: process.Diagnostics, Workloads: process.Workloads,
		Hosting: process.Hosting, Data: process.Content, Transfers: process.Transfers,
	}, token)
}
