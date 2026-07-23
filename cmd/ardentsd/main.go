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
	"ardents/internal/identity/principal"
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
	owner, err := applicationContentOwner(call)
	if err != nil {
		return contentdomain.Blob{}, err
	}
	return s.owners.Node.PublishBlobForOwner(owner, command)
}

func (s applicationContentStore) GetBlob(call applicationcall.Call, id string) (contentdomain.Blob, bool) {
	owner, err := applicationContentOwner(call)
	if err != nil {
		return contentdomain.Blob{}, false
	}
	return s.owners.Content.GetBlobForOwner(owner, id)
}

func (s applicationContentStore) GetBlobPayload(call applicationcall.Call, id string) ([]byte, error) {
	owner, err := applicationContentOwner(call)
	if err != nil {
		return nil, contentdomain.ErrBlobNotFound
	}
	return s.owners.Content.GetBlobPayloadForOwner(owner, id)
}

func (s applicationContentStore) FetchBlob(ctx context.Context, call applicationcall.Call, id string) (contentdomain.Blob, error) {
	owner, err := applicationContentOwner(call)
	if err != nil || !s.owners.Content.HasBlobOwner(owner, id) {
		return contentdomain.Blob{}, contentdomain.ErrBlobNotFound
	}
	blob, err := s.owners.Node.FetchBlob(ctx, id)
	if err != nil {
		return contentdomain.Blob{}, err
	}
	if blob.Reference.String() != id || !s.owners.Content.HasBlobOwner(owner, id) {
		return contentdomain.Blob{}, contentdomain.ErrBlobNotFound
	}
	return blob, nil
}

func applicationContentOwner(call applicationcall.Call) (principal.ID, error) {
	if !call.IsPrincipal() {
		return principal.ID{}, contentdomain.ErrBlobNotFound
	}
	owner, err := principal.Parse(call.Effective())
	if err != nil {
		return principal.ID{}, contentdomain.ErrBlobNotFound
	}
	return owner, nil
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
