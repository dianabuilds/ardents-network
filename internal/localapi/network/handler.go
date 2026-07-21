package network

import (
	"ardents/internal/discovery"
	localauth "ardents/internal/localapi/auth"
	domain "ardents/internal/network"
	"ardents/internal/publication"
)

type Discovery interface {
	ResolveRecord(string, string) (discovery.ResolutionResult, error)
	ResolveService(string) (discovery.ServiceResult, error)
}

type Records interface {
	ListRecords() ([]discovery.CatalogRecordSnapshot, error)
	ImportRecord(discovery.CatalogRecordSnapshot) (discovery.RecordImportResult, error)
}

type Status interface {
	GetNetworkStatus() domain.StatusSnapshot
	GetDiscoveryStatus() discovery.StatusSnapshot
	GetLocalPresence() publication.LocalPresenceSnapshot
	ListPeers() []discovery.PeerSnapshot
	ListRouteCandidates(discovery.ListRouteCandidatesQuery) ([]discovery.RouteCandidateSnapshot, discovery.RouteSnapshot, error)
}

type API struct {
	discovery Discovery
	records   Records
	status    Status
	auth      localauth.Config
}

func NewHandler(discovery Discovery, records Records, status Status, auth localauth.Config) *API {
	return &API{discovery: discovery, records: records, status: status, auth: auth}
}
