package diagnostics

import (
	"fmt"
	"strings"
)

const DiscoveryRefreshFailedCode = "discovery.refresh_failed"

func AllowsObservedSync(state, ready, degraded string) bool {
	return state == ready || state == degraded
}

func lifecycleForHealth(state string) string {
	switch state {
	case HealthFailed:
		return Failed
	case HealthDegraded:
		return Degraded
	default:
		return Ready
	}
}

func RuntimeFailure(action string, failed bool, detail string) error {
	if !failed {
		return nil
	}
	if detail != "" {
		return fmt.Errorf("node %s failed: %s", action, detail)
	}
	return fmt.Errorf("node %s failed", action)
}

func AdoptPrimaryReason(recorder *Recorder, domain, state string, reason *Reason) {
	current := recorder.Health().PrimaryReason
	if current != nil && current.Domain != domain {
		return
	}
	recorder.SetPrimary(state, reason)
}

func RestorePrimaryReason(recorder *Recorder, domain string) {
	current := recorder.Health().PrimaryReason
	if current == nil || current.Domain != domain {
		return
	}
	recorder.ClearPrimary()
	PromoteSubsystemPrimary(recorder, "")
}

func PromoteSubsystemPrimary(recorder *Recorder, domain string) {
	for _, item := range recorder.Health().Subsystems {
		if (domain == "" || item.Domain == domain) && item.Reason != nil {
			recorder.SetPrimary(item.State, item.Reason)
			return
		}
	}
}

func SyncBootHealth(recorder *Recorder, state, reason string) {
	switch state {
	case "ready":
		recorder.SetSubsystem("boot", HealthReady, nil)
	case "degraded":
		recorder.SetSubsystem("boot", HealthDegraded, &Reason{Code: "boot.join.degraded", Domain: "boot", Summary: "bootstrap did not complete cleanly", Detail: reason, Impact: "node remains controllable with limited network confidence", Recovery: "operator", OperatorActionRequired: true})
	default:
		recorder.ClearSubsystem("boot")
	}
}

func SyncPrimaryReason(recorder *Recorder) {
	health := recorder.Health()
	if health.PrimaryReason != nil && health.PrimaryReason.Domain != "boot" && health.PrimaryReason.Domain != "transport" {
		return
	}
	for _, item := range health.Subsystems {
		if (item.Domain == "boot" || item.Domain == "transport") && item.Reason != nil {
			recorder.SetPrimary(item.State, item.Reason)
			return
		}
	}
	recorder.ClearPrimary()
}

func SyncLifecycleState(recorder *Recorder, move func(string)) {
	move(lifecycleForHealth(recorder.Health().State))
}

func SubsystemReasonCode(summary HealthSummary, domain string) string {
	for _, item := range summary.Subsystems {
		if item.Domain == domain && item.Reason != nil {
			return item.Reason.Code
		}
	}
	return ""
}

func CurrentPrimaryReasonCode(recorder *Recorder) string {
	if reason := recorder.Health().PrimaryReason; reason != nil {
		return reason.Code
	}
	return ""
}

func ApplyTransportHealth(recorder *Recorder, state, reason, profile, mode string) {
	rawReason := reason
	if reason != "" {
		reason = "profile " + profile + ", mode " + mode + ": " + reason
	}
	if state == "ready" {
		recorder.SetSubsystem("transport", HealthReady, nil)
		return
	}
	code, summary, impact, recovery, action := "transport.bootstrap.degraded", "transport is not operational on the relay path", "network messaging is not ready", "operator", true
	if mode == "restricted_defense" && rawReason == "restricted defense mode is active" {
		code, summary, impact, recovery, action = "transport.mode.restricted_defense", "transport is in restricted defense recovery cooldown", "network messaging remains constrained until the recovery cooldown completes", "automatic", false
	} else if state != "degraded" {
		code, summary, impact = "transport.state.unready", "transport is not ready", "network messaging is unavailable"
	}
	recorder.SetSubsystem("transport", HealthDegraded, &Reason{Code: code, Domain: "transport", Summary: summary, Detail: reason, Impact: impact, Recovery: recovery, OperatorActionRequired: action})
}

func ClearRuntimeHealthForStop(recorder *Recorder) {
	for _, domain := range []string{"boot", "transport", "discovery", "workload", "publication"} {
		recorder.ClearSubsystem(domain)
	}
	if strings.HasPrefix(SubsystemReasonCode(recorder.Health(), "data"), "privacy.capability.") {
		recorder.ClearSubsystem("data")
	}
	primary := recorder.Health().PrimaryReason
	if primary == nil {
		return
	}
	shouldClearPrimary := primary.Domain == "boot" || primary.Domain == "transport" || primary.Domain == "discovery" || primary.Domain == "workload" || primary.Domain == "publication"
	if shouldClearPrimary || (primary.Domain == "data" && strings.HasPrefix(primary.Code, "privacy.capability.")) {
		recorder.ClearPrimary()
		PromoteSubsystemPrimary(recorder, "")
	}
}

func RecordDiscoveryRefreshFailure(recorder *Recorder, name string, err error, degrade func(string), adopt func(string, string, *Reason), move func(string), publish func(string, map[string]any)) {
	degrade(err.Error())
	reason := &Reason{Code: DiscoveryRefreshFailedCode, Domain: "discovery", Summary: "discovery refresh failed", Detail: err.Error(), Impact: "local publication or remote discovery knowledge may remain stale", Recovery: "operator", OperatorActionRequired: true, Resource: name}
	recorder.SetSubsystem("discovery", HealthDegraded, reason)
	adopt("discovery", HealthDegraded, reason)
	move(lifecycleForHealth(recorder.Health().State))
	publish("discovery.refresh_failed", map[string]any{"id": name, "error": err.Error()})
	recorder.RecordEvent("discovery", "refresh_failed", name, reason.Summary, DiscoveryRefreshFailedCode, map[string]any{"detail": err.Error()})
}

func ClearDiscoveryRefreshFailure(recorder *Recorder, ready func(), restore func(string), move func(string)) {
	if SubsystemReasonCode(recorder.Health(), "discovery") == DiscoveryRefreshFailedCode {
		recorder.ClearSubsystem("discovery")
		restore("discovery")
		ready()
	}
	SyncPrimaryReason(recorder)
	move(lifecycleForHealth(recorder.Health().State))
}
