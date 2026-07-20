package recovery

import (
	"fmt"

	"ardents/internal/diagnostics"
)

func DegradeDiscoveryPrivacy(
	diag *diagnostics.Recorder,
	resource string,
	code string,
	rejected int,
	setDiscoveryDegraded func(string),
	adoptPrimary func(domain string, state string, reason *diagnostics.Reason),
	moveLifecycle func(string),
) {
	detail := fmt.Sprintf("private discovery channel rejected %d envelope(s)", rejected)
	if rejected == 0 {
		detail = "private discovery channel is unavailable"
	}
	reason := &diagnostics.Reason{
		Code: code, Domain: "discovery", Summary: "private discovery is degraded",
		Detail: detail, Impact: "remote discovery may be incomplete; retained records remain usable",
		Recovery: "operator", OperatorActionRequired: true, Resource: resource,
	}
	setDiscoveryDegraded(code)
	diag.SetSubsystem("discovery", diagnostics.HealthDegraded, reason)
	if diag.Health().State != diagnostics.HealthFailed {
		adoptPrimary("discovery", diagnostics.HealthDegraded, reason)
		moveLifecycle("degraded")
	}
	diag.RecordEvent("discovery", "privacy_degraded", resource, reason.Summary, code, map[string]any{"rejected": rejected})
}
