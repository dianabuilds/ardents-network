package policy

import (
	identityapi "ardents/internal/identity"
)

type CapabilityConfig struct {
	DisablePrivateCapabilityUse bool
	DeniedCapabilityScopes      []string
}

func CheckCapabilityUse(cfg CapabilityConfig, use identityapi.CapabilityUse) Result {
	if cfg.DisablePrivateCapabilityUse {
		return Deny("policy_capability_denied", "private capability use is disabled by policy")
	}
	if ContainsNormalized(cfg.DeniedCapabilityScopes, string(use.Scope)) {
		return Deny("policy_capability_denied", "capability scope is denied by policy")
	}
	return Allow()
}
