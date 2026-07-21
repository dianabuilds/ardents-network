package network

import "time"

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
