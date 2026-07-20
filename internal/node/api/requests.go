package api

type NodeStartCommand struct{}
type NodeStopCommand struct{}
type NodeStatusQuery struct{}
type NodeCapabilitiesQuery struct{}
type GetNodeRuntimeQuery struct{}
type GetDiscoveryStatusQuery struct{}
type GetLocalPresenceQuery struct{}
type ListPeersQuery struct{}
type GetNetworkStatusQuery struct{}

type ListRouteCandidatesQuery struct {
	Resource string `json:"resource,omitempty"`
	Subject  string `json:"subject,omitempty"`
	Kind     string `json:"kind,omitempty"`
	Service  string `json:"service,omitempty"`
}
