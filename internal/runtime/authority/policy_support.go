package authority

import (
	hostingexposure "ardents/internal/hosting/exposure"
	policyapi "ardents/internal/policy/api"
	"ardents/internal/workload/observedstate"
)

func effectiveWorkloadStatus(item observedstate.Status, policy policyapi.Service) observedstate.Status {
	if policy == nil {
		return item
	}
	item.PublishedServices = hostingexposure.EffectivePublishedServices(item.PublishedServices, policy.AllowServicePublication)
	return item
}
