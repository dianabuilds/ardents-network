package evaluation

import (
	domainnetwork "ardents/internal/network/api"
	"ardents/internal/policy/decision"
	"ardents/internal/policy/rule"
)

type RouteConfig struct {
	DisableUntrustedRouteUse bool
	DeniedRouteSchemes       []string
}

func CheckRouteUse(cfg RouteConfig, candidate domainnetwork.Candidate) decision.Result {
	if cfg.DisableUntrustedRouteUse && !candidate.Trusted {
		return decision.Deny("policy_route_denied", "untrusted routes are disabled by policy")
	}
	if rule.ContainsNormalized(cfg.DeniedRouteSchemes, candidate.Scheme) {
		return decision.Deny("policy_route_denied", "route scheme is denied by policy")
	}
	return decision.Allow()
}
