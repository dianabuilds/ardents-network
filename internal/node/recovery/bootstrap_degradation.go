package recovery

import (
	"ardents/internal/diagnostics"
)

func DegradeTransportBootstrap(
	diag *diagnostics.Recorder,
	cfgName string,
	code, summary, detail, impact string,
	payload map[string]any,
	adoptPrimary func(domain string, state string, reason *diagnostics.Reason),
	moveLifecycle func(string),
) {
	reason := &diagnostics.Reason{
		Code:                   code,
		Domain:                 "transport",
		Summary:                summary,
		Detail:                 detail,
		Impact:                 impact,
		Recovery:               "operator",
		OperatorActionRequired: true,
		Resource:               cfgName,
	}
	diag.SetSubsystem("transport", diagnostics.HealthDegraded, reason)
	if diag.Health().State != diagnostics.HealthFailed {
		adoptPrimary("transport", diagnostics.HealthDegraded, reason)
		moveLifecycle("degraded")
	}
	diag.RecordEvent("transport", "degraded", cfgName, summary, code, cloneMap(payload))
}

func DegradeDiscoveryImport(
	diag *diagnostics.Recorder,
	recordID string,
	detail string,
	setDiscoveryDegraded func(string),
	adoptPrimary func(domain string, state string, reason *diagnostics.Reason),
	moveLifecycle func(string),
) {
	summary := "bootstrap discovery import was degraded"
	reason := &diagnostics.Reason{
		Code:                   "discovery.bootstrap.import_degraded",
		Domain:                 "discovery",
		Summary:                summary,
		Detail:                 detail,
		Impact:                 "remote discovery catalog is incomplete",
		Recovery:               "operator",
		OperatorActionRequired: true,
		Resource:               recordID,
	}
	setDiscoveryDegraded(detail)
	diag.SetSubsystem("discovery", diagnostics.HealthDegraded, reason)
	if diag.Health().State != diagnostics.HealthFailed {
		adoptPrimary("discovery", diagnostics.HealthDegraded, reason)
		moveLifecycle("degraded")
	}
	diag.RecordEvent("discovery", "bootstrap_import_degraded", recordID, summary, reason.Code, map[string]any{"detail": detail})
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
