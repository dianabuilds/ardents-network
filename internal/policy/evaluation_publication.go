package policy

import (
	hostingservice "ardents/internal/workload/registry"
	"strings"
)

type ServicePublicationConfig struct {
	DisableServicePublication       bool
	DisableNetworkPublishedServices bool
	DeniedServiceTypes              []string
}

func CheckServicePublication(cfg ServicePublicationConfig, spec hostingservice.ServiceSpec) Result {
	if cfg.DisableServicePublication {
		return Deny("policy_publication_denied", "service publication is disabled by policy")
	}
	if strings.EqualFold(spec.Mode, "NetworkPublished") && cfg.DisableNetworkPublishedServices {
		return Deny("policy_publication_denied", "network-published services are disabled by policy")
	}
	if ContainsNormalized(cfg.DeniedServiceTypes, spec.Type) {
		return Deny("policy_publication_denied", "service type is denied by policy")
	}
	return Allow()
}
