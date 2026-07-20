package evaluation

import (
	"strings"

	hostingservice "ardents/internal/hosting/service"
	"ardents/internal/policy/decision"
	"ardents/internal/policy/rule"
)

type ServicePublicationConfig struct {
	DisableServicePublication       bool
	DisableNetworkPublishedServices bool
	DeniedServiceTypes              []string
}

func CheckServicePublication(cfg ServicePublicationConfig, spec hostingservice.Spec) decision.Result {
	if cfg.DisableServicePublication {
		return decision.Deny("policy_publication_denied", "service publication is disabled by policy")
	}
	if strings.EqualFold(spec.Mode, "NetworkPublished") && cfg.DisableNetworkPublishedServices {
		return decision.Deny("policy_publication_denied", "network-published services are disabled by policy")
	}
	if rule.ContainsNormalized(cfg.DeniedServiceTypes, spec.Type) {
		return decision.Deny("policy_publication_denied", "service type is denied by policy")
	}
	return decision.Allow()
}
