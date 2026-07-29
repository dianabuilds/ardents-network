package network

import "time"

const (
	// PrivateLANProbeMaxAge bounds how long one cross-host dial observation may
	// keep a translated-host LAN endpoint publishable.
	PrivateLANProbeMaxAge = 2 * time.Minute
	// PrivateLANProbeFutureSkew permits only the accepted bounded clock error.
	PrivateLANProbeFutureSkew = 30 * time.Second
)

type ReachabilityMode string

const (
	ReachabilityLocalOnly    ReachabilityMode = "local_only"
	ReachabilityPrivateLAN   ReachabilityMode = "private_lan"
	ReachabilityPublicDirect ReachabilityMode = "public_direct"
	ReachabilityOutboundOnly ReachabilityMode = "outbound_only"
)

type ReachabilitySnapshot struct {
	Mode       ReachabilityMode
	State      string
	Reason     string
	Reachable  bool
	ObservedAt time.Time
}

// PrivateLANProbe is protected, source-attributable deployment evidence. It
// establishes LAN scope only and must not be copied into ordinary status.
type PrivateLANProbe struct {
	SourceSlot string
	TargetSlot string
	Address    string
	ObservedAt time.Time
	Success    bool
}

// PrivateLANReachability is the optional transport boundary used by the
// private-LAN deployment adapter without widening the general Service API.
type PrivateLANReachability interface {
	ApplyPrivateLANProbe(PrivateLANProbe) error
}

func NormalizeReachabilityMode(mode ReachabilityMode) ReachabilityMode {
	if mode == "" {
		return ReachabilityPrivateLAN
	}
	return mode
}

func ValidReachabilityMode(mode ReachabilityMode) bool {
	switch NormalizeReachabilityMode(mode) {
	case ReachabilityLocalOnly, ReachabilityPrivateLAN, ReachabilityPublicDirect, ReachabilityOutboundOnly:
		return true
	default:
		return false
	}
}
