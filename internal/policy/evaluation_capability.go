package policy

import (
	identityapi "ardents/internal/identity"
)

type ChannelGrantPolicyConfig struct {
	DisablePrivateChannelGrantUse bool
	DeniedChannelGrantScopes      []string
}

func CheckChannelGrantUse(cfg ChannelGrantPolicyConfig, use identityapi.CapabilityUse) Result {
	if cfg.DisablePrivateChannelGrantUse {
		return Deny("policy_channel_grant_denied", "private channel grant use is disabled by policy")
	}
	if ContainsNormalized(cfg.DeniedChannelGrantScopes, string(use.Scope)) {
		return Deny("policy_channel_grant_denied", "channel grant scope is denied by policy")
	}
	return Allow()
}
