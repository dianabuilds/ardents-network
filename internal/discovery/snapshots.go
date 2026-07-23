package discovery

import "time"

type CatalogRecordSnapshot struct {
	Version   uint32                       `json:"version"`
	Node      *CatalogNodeFactsSnapshot    `json:"node,omitempty"`
	Service   *CatalogServiceFactsSnapshot `json:"service,omitempty"`
	IssuedAt  time.Time                    `json:"issued_at"`
	ExpiresAt time.Time                    `json:"expires_at"`
	Signature string                       `json:"signature,omitempty"`
	Source    string                       `json:"source,omitempty"`
}

type CatalogNodeFactsSnapshot struct {
	Principal string   `json:"principal,omitempty"`
	PublicKey string   `json:"public_key,omitempty"`
	Endpoints []string `json:"endpoints,omitempty"`
}

type CatalogServiceFactsSnapshot struct {
	ID            string   `json:"service_id,omitempty"`
	Type          string   `json:"service_type,omitempty"`
	NodePrincipal string   `json:"node_principal,omitempty"`
	WorkloadID    string   `json:"workload_id,omitempty"`
	Mode          string   `json:"mode,omitempty"`
	PublicKey     string   `json:"public_key,omitempty"`
	Endpoints     []string `json:"endpoints,omitempty"`
}

func (r CatalogRecordSnapshot) RecordID() string {
	if r.Node != nil {
		return r.Node.Principal + ":node"
	}
	if r.Service != nil {
		return r.Service.ID
	}
	return ""
}

func (r CatalogRecordSnapshot) Subject() string {
	if r.Node != nil {
		return r.Node.Principal
	}
	if r.Service != nil {
		return r.Service.ID
	}
	return ""
}

func (r CatalogRecordSnapshot) Kind() string {
	if r.Node != nil && r.Service == nil {
		return "node"
	}
	if r.Service != nil && r.Node == nil {
		return "service"
	}
	return ""
}

func (r CatalogRecordSnapshot) EndpointList() []string {
	if r.Node != nil {
		return r.Node.Endpoints
	}
	if r.Service != nil {
		return r.Service.Endpoints
	}
	return nil
}

type TransportTarget struct {
	Subject     string `json:"subject,omitempty"`
	Service     string `json:"service,omitempty"`
	Endpoint    string `json:"endpoint,omitempty"`
	Scheme      string `json:"scheme,omitempty"`
	Mode        string `json:"mode,omitempty"`
	Trusted     bool   `json:"trusted,omitempty"`
	Usable      bool   `json:"usable,omitempty"`
	Cost        int    `json:"cost,omitempty"`
	Privacy     int    `json:"privacy,omitempty"`
	Reliability int    `json:"reliability,omitempty"`
}

type RouteSnapshot struct {
	Outcome    string           `json:"outcome,omitempty"`
	Reason     string           `json:"reason,omitempty"`
	Candidates int              `json:"candidates,omitempty"`
	Usable     int              `json:"usable,omitempty"`
	Selected   *TransportTarget `json:"selected,omitempty"`
}

type ResolutionResult struct {
	Outcome    string                `json:"outcome,omitempty"`
	Source     string                `json:"source,omitempty"`
	Record     CatalogRecordSnapshot `json:"record"`
	Trust      TrustSnapshot         `json:"trust"`
	Route      RouteSnapshot         `json:"route"`
	Candidates []TransportTarget     `json:"candidates,omitempty"`
}

type ServiceResult struct {
	Service string             `json:"service,omitempty"`
	Outcome string             `json:"outcome,omitempty"`
	Route   RouteSnapshot      `json:"route"`
	Matches []ResolutionResult `json:"matches,omitempty"`
}

type RecordImportResult struct {
	State    string `json:"state,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Accepted bool   `json:"accepted,omitempty"`
}

type RouteCandidateSnapshot struct {
	Subject     string `json:"subject,omitempty"`
	Service     string `json:"service,omitempty"`
	Endpoint    string `json:"endpoint,omitempty"`
	Scheme      string `json:"scheme,omitempty"`
	Mode        string `json:"mode,omitempty"`
	Trusted     bool   `json:"trusted,omitempty"`
	Usable      bool   `json:"usable,omitempty"`
	Cost        int    `json:"cost,omitempty"`
	Privacy     int    `json:"privacy,omitempty"`
	Reliability int    `json:"reliability,omitempty"`
	State       string `json:"state,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

type ListRouteCandidatesQuery struct {
	Resource string
	Subject  string
	Kind     string
	Service  string
}

type TrustSnapshot struct {
	State   string `json:"state,omitempty"`
	Outcome string `json:"outcome,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Valid   bool   `json:"valid,omitempty"`
	Trusted bool   `json:"trusted,omitempty"`
	Usable  bool   `json:"usable,omitempty"`
}

type StatusSnapshot struct {
	State           string
	Reason          string
	LocalRecords    int
	RemoteRecords   int
	TrustedRecords  int
	RejectedRecords int
	StaleRecords    int
	LastPublishAt   time.Time
	LastRefreshAt   time.Time
}

type SummarySnapshot struct {
	State     string
	Reason    string
	Records   int
	LocalNode string
	Services  int
}

type PeerSnapshot struct {
	NodeID       string
	Addresses    []string
	Trust        TrustSnapshot
	Reachability string
	Source       string
	LastSeenAt   time.Time
	State        string
	Reason       string
}
