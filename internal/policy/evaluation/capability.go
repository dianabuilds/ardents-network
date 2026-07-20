package evaluation

import (
	identityapi "ardents/internal/identity/api"
	"ardents/internal/policy/decision"
	"ardents/internal/policy/rule"
)

type CapabilityConfig struct {
	DisablePrivateCapabilityUse bool
	DeniedCapabilityScopes      []string
}

func CheckCapabilityUse(cfg CapabilityConfig, use identityapi.CapabilityUse) decision.Result {
	if cfg.DisablePrivateCapabilityUse {
		return decision.Deny("policy_capability_denied", "private capability use is disabled by policy")
	}
	if rule.ContainsNormalized(cfg.DeniedCapabilityScopes, string(use.Scope)) {
		return decision.Deny("policy_capability_denied", "capability scope is denied by policy")
	}
	return decision.Allow()
}
