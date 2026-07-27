package localapi

import (
	"fmt"
	"net/http"

	identityaccess "ardents/internal/identity/access"
	identityhandler "ardents/internal/localapi/identity"
)

// NewProtectedHandler exposes only Principal identity and product calls on the
// Unix-only Operator endpoint.
func NewProtectedHandler(deps Dependencies, access *identityaccess.Service, node string, peer [32]byte, source identityaccess.SourceKey) (string, http.Handler, error) {
	if access == nil {
		return "", nil, fmt.Errorf("Principal access service is required")
	}
	identityPath, identity, err := identityhandler.NewHandler(access, node, peer, source)
	if err != nil {
		return "", nil, err
	}
	principal, err := NewPrincipalHandler(deps, access, node, peer, source)
	if err != nil {
		return "", nil, err
	}
	return "/", newProtectedMux(identityPath, identity, principal), nil
}

func newProtectedMux(identityPath string, identity, principal http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(identityPath, identity)
	mux.Handle("/ardents.v1.NodeService/", principal)
	mux.Handle("/ardents.v1.AuthorityService/", principal)
	mux.Handle("/ardents.v1.ConfigurationService/", principal)
	mux.Handle("/ardents.v1.NetworkService/", principal)
	mux.Handle("/ardents.v1.DiagnosticsService/", principal)
	mux.Handle("/ardents.v1.WorkloadService/", principal)
	mux.Handle("/ardents.v1.ContentService/", principal)
	mux.Handle("/ardents.v1.TransferService/", principal)
	mux.Handle("/ardents.v1.RetentionService/", principal)
	return mux
}
