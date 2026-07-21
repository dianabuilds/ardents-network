package discovery

import "time"

type CatalogRecordSnapshot struct {
	ID        string    `json:"id,omitempty"`
	Kind      string    `json:"kind,omitempty"`
	Subject   string    `json:"subject,omitempty"`
	Node      string    `json:"node,omitempty"`
	Device    string    `json:"device,omitempty"`
	Owner     string    `json:"owner,omitempty"`
	Service   string    `json:"service,omitempty"`
	Mode      string    `json:"mode,omitempty"`
	PublicKey string    `json:"public_key,omitempty"`
	Endpoints []string  `json:"endpoints,omitempty"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Signature string    `json:"signature,omitempty"`
	Source    string    `json:"source,omitempty"`
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
	DeviceID     string
	Addresses    []string
	Trust        TrustSnapshot
	Reachability string
	Source       string
	LastSeenAt   time.Time
	State        string
	Reason       string
}
