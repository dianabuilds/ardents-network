package readiness

import (
	"ardents/internal/diagnostics"
	transport "ardents/internal/network/api"
)

func ApplyTransportHealth(diag *diagnostics.Recorder, state, reason string, snapshot transport.Snapshot) {
	rawReason := reason
	profile := string(snapshot.Profile)
	mode := string(snapshot.Mode)
	if reason != "" {
		reason = "profile " + profile + ", mode " + mode + ": " + reason
	}
	switch state {
	case "ready":
		diag.SetSubsystem("transport", diagnostics.HealthReady, nil)
	case "degraded":
		code := "transport.bootstrap.degraded"
		summary := "transport is not operational on the relay path"
		impact := "network messaging is not ready"
		recovery := "operator"
		operatorActionRequired := true
		if snapshot.Mode == transport.ModeRestrictedDefense && rawReason == "restricted defense mode is active" {
			code = "transport.mode.restricted_defense"
			summary = "transport is in restricted defense recovery cooldown"
			impact = "network messaging remains constrained until the recovery cooldown completes"
			recovery = "automatic"
			operatorActionRequired = false
		}
		diag.SetSubsystem("transport", diagnostics.HealthDegraded, &diagnostics.Reason{
			Code:                   code,
			Domain:                 "transport",
			Summary:                summary,
			Detail:                 reason,
			Impact:                 impact,
			Recovery:               recovery,
			OperatorActionRequired: operatorActionRequired,
		})
	default:
		diag.SetSubsystem("transport", diagnostics.HealthDegraded, &diagnostics.Reason{
			Code:                   "transport.state.unready",
			Domain:                 "transport",
			Summary:                "transport is not ready",
			Detail:                 reason,
			Impact:                 "network messaging is unavailable",
			Recovery:               "operator",
			OperatorActionRequired: true,
		})
	}
}
