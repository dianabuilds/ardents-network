package publication

import "ardents/internal/workload/execution"

func effectiveWorkloadStatus(item execution.Status, policy Policy) execution.Status {
	if policy == nil {
		return item
	}
	item.PublishedServices = EffectivePublishedServices(item.PublishedServices, policy.AllowServicePublication)
	return item
}

func deniedPayload(resource, action string, err error) map[string]any {
	reason := ""
	if err != nil {
		reason = err.Error()
	}
	return map[string]any{
		"id":       resource,
		"action":   action,
		"reason":   reason,
		"resource": resource,
	}
}
