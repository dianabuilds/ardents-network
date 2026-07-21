package policy

import (
	domainnetwork "ardents/internal/network"
)

type RouteConfig struct {
	DisableUntrustedRouteUse bool
	DeniedRouteSchemes       []string
}

func CheckRouteUse(cfg RouteConfig, candidate domainnetwork.Candidate) Result {
	if cfg.DisableUntrustedRouteUse && !candidate.Trusted {
		return Deny("policy_route_denied", "untrusted routes are disabled by policy")
	}
	if ContainsNormalized(cfg.DeniedRouteSchemes, candidate.Scheme) {
		return Deny("policy_route_denied", "route scheme is denied by policy")
	}
	return Allow()
}
