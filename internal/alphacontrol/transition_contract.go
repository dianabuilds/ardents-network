package alphacontrol

// TransitionDomain is one independent H4-6B control decision. It is a
// disclosure vocabulary only and never grants an Endpoint authority.
type TransitionDomain string

const (
	DomainReleaseSafety            TransitionDomain = "release-safety"
	DomainNetworkEpoch             TransitionDomain = "network-epoch"
	DomainCompatibility            TransitionDomain = "compatibility"
	DomainNamespaceMaterialization TransitionDomain = "namespace-materialization"
)

// TransitionContract describes the alpha transition boundary for one domain.
// Every field is participant-visible so an omitted or failed input is not
// silently reinterpreted as authority for another domain.
type TransitionContract struct {
	Domain          TransitionDomain `json:"domain"`
	Selected        bool             `json:"selected"`
	AuthorityRoot   string           `json:"authority_root"`
	Predecessor     string           `json:"predecessor"`
	Freshness       string           `json:"freshness"`
	Rotation        string           `json:"rotation"`
	Revocation      string           `json:"revocation"`
	RollbackFloor   string           `json:"rollback_floor"`
	EmergencyAction string           `json:"emergency_action"`
	UserFailure     string           `json:"user_failure"`
	Evidence        string           `json:"evidence"`
}

// H46BTransitionContracts returns the fixed alpha disclosure. Release,
// Network, and Compatibility retain their own authoritative mechanisms.
// Namespace is deliberately unselected for this profile: no project control
// statement can stand in for an authenticated global close.
func H46BTransitionContracts() []TransitionContract {
	return []TransitionContract{
		{
			Domain: DomainReleaseSafety, Selected: true,
			AuthorityRoot:   "Release trusted-root chain pinned by enrollment",
			Predecessor:     "the retained Release floor and consecutive trusted-root chain",
			Freshness:       "authenticated timestamp and Release Safety deadlines",
			Rotation:        "consecutive authenticated Release root rotation retained in the Release floor",
			Revocation:      "a Release decision may revoke the build or require replacement",
			RollbackFloor:   "the Endpoint-owned non-decreasing Release floor",
			EmergencyAction: "stop new work or terminate at the authenticated Release Safety deadline",
			UserFailure:     "release unsafe, revoked, expired, conflicting, or unavailable",
			Evidence:        "exact TUF-compatible metadata, artifact digest, Release component, and retained floor",
		},
		{
			Domain: DomainNetworkEpoch, Selected: true,
			AuthorityRoot:   "Network Epoch authority set and threshold pinned by enrolled Network evidence",
			Predecessor:     "the persisted State Epoch successor relation",
			Freshness:       "the authenticated Epoch validity bound and Time Confidence",
			Rotation:        "a verified State successor may replace the authenticated authority/profile facts",
			Revocation:      "a State successor withdraws expired or no-longer-eligible duties",
			RollbackFloor:   "the State-owned non-decreasing Epoch root",
			EmergencyAction: "refuse new State-dependent work and drain or withdraw affected duties",
			UserFailure:     "network state unavailable, expired, invalid, replayed, or conflicting",
			Evidence:        "exact Epoch bytes, authority identifiers, threshold signatures, material roots, and State floor",
		},
		{
			Domain: DomainCompatibility, Selected: true,
			AuthorityRoot:   "the independently pinned Compatibility component bound to accepted Release and Network identities",
			Predecessor:     "the accepted Release identity and Network Epoch/profile tuple",
			Freshness:       "the earlier of the bound Release Safety and Network Epoch validity limits",
			Rotation:        "a higher catalog/component generation bound to a successor Release and Epoch tuple",
			Revocation:      "a revoked Release or incompatible Epoch/profile invalidates the tuple",
			RollbackFloor:   "catalog/component floors together with retained Release and State floors",
			EmergencyAction: "refuse the incompatible profile; never silently downgrade a Route Profile",
			UserFailure:     "build incompatible or control tuple unavailable, stale, forged, replayed, or conflicting",
			Evidence:        "exact Compatibility statement and the bound Release digest/build/protocol and Network digest/Epoch/profile",
		},
		{
			Domain: DomainNamespaceMaterialization, Selected: false,
			AuthorityRoot:   "none in the current Functional Alpha profile",
			Predecessor:     "none; no authenticated global close or current Namespace successor is selected",
			Freshness:       "not applicable because no Namespace materialization input is accepted",
			Rotation:        "not applicable; alpha control cannot rotate a Namespace authority",
			Revocation:      "not applicable; alpha control cannot revoke a Name or Namespace Record",
			RollbackFloor:   "not applicable; no current Namespace materialization floor is created",
			EmergencyAction: "do not materialize, release, or reclaim a Namespace",
			UserFailure:     "Namespace materialization is unavailable in this alpha profile",
			Evidence:        "the absent Namespace authority/component and the explicit H4-6B deferral record",
		},
	}
}

// H46BComponent reports the independently verified ACA1 component for one
// selected transition domain. Namespace materialization has no component in
// the current Functional Alpha profile.
func H46BComponent(domain TransitionDomain) (ComponentClass, bool) {
	switch domain {
	case DomainReleaseSafety:
		return ComponentRelease, true
	case DomainNetworkEpoch:
		return ComponentNetwork, true
	case DomainCompatibility:
		return ComponentCompatibility, true
	default:
		return 0, false
	}
}
