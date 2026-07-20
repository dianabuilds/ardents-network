package controller

import (
	"fmt"
	"time"

	"ardents/internal/workload/desiredstate"
	"ardents/internal/workload/execution"
	"ardents/internal/workload/observedstate"
	workloadregistry "ardents/internal/workload/registry"
)

func RejectAdmission(item observedstate.Status, now time.Time, err error) observedstate.Status {
	item.Observed = observedstate.Failed
	item.Reason = "admission rejected: " + err.Error()
	item.NeedsOperatorAction = true
	item.LastTransitionAt = now
	item.PublishedServices = workloadregistry.ServiceStatuses(item.Spec, false, item.Reason)
	return item
}

func ReconcilePresent(item observedstate.Status, now time.Time) observedstate.Status {
	item.Observed = observedstate.Accepted
	item.Reason = ""
	item.NeedsOperatorAction = false
	item.Instance = execution.Instance{}
	item.LastTransitionAt = now
	item.PublishedServices = workloadregistry.ServiceStatuses(item.Spec, false, "workload not running")
	return item
}

func MarkRunning(item observedstate.Status, now time.Time) observedstate.Status {
	item.Observed = observedstate.Running
	item.Reason = ""
	item.NeedsOperatorAction = false
	item.LastTransitionAt = now
	item.PublishedServices = workloadregistry.ServiceStatuses(item.Spec, true, "")
	return item
}

func DegradedRunningAgainstDesired(item observedstate.Status, instance execution.Instance, now time.Time) (observedstate.Status, bool) {
	if item.Spec.Desired != desiredstate.Stopped && item.Spec.Desired != desiredstate.Disabled {
		return item, false
	}
	item.Instance = instance
	item.Observed = observedstate.Degraded
	item.Reason = "workload is still running after stop request"
	item.NeedsOperatorAction = true
	item.LastTransitionAt = now
	item.PublishedServices = workloadregistry.ServiceStatuses(item.Spec, false, item.Reason)
	return item, true
}

func (s *Service) observedAfterUnexpectedExit(item observedstate.Status, now time.Time) observedstate.Status {
	item.Instance.Running = false
	item.Instance.PID = 0
	if item.Instance.FinishedAt.IsZero() {
		item.Instance.FinishedAt = now
	}
	if item.Instance.Reason == "" {
		item.Instance.Reason = "process exited before observation sync"
	}
	switch item.Spec.Desired {
	case desiredstate.Running:
		item.RestartCount++
		item.Reason = unexpectedExitReason(item.Instance)
		terminal := item.Instance.OOMKilled || item.Spec.RestartPolicy == "never" || item.RestartCount > s.restartBudget
		if terminal {
			item.Observed = observedstate.Failed
			item.NeedsOperatorAction = true
		} else {
			item.Observed = observedstate.Degraded
			item.NeedsOperatorAction = false
			item.Reason += "; restart reconciliation pending"
		}
		item.PublishedServices = workloadregistry.ServiceStatuses(item.Spec, false, item.Reason)
	case desiredstate.Present:
		item.Observed = observedstate.Accepted
		item.Reason = ""
		item.NeedsOperatorAction = false
		item.PublishedServices = workloadregistry.ServiceStatuses(item.Spec, false, "workload not running")
	default:
		item.Observed = observedstate.Stopped
		item.Reason = ""
		item.NeedsOperatorAction = false
		item.PublishedServices = workloadregistry.ServiceStatuses(item.Spec, false, "workload not running")
	}
	item.LastTransitionAt = now
	return item
}

func unexpectedExitReason(instance execution.Instance) string {
	if instance.OOMKilled {
		return "workload exceeded its memory limit"
	}
	if instance.ExitCode != nil {
		return fmt.Sprintf("workload exited with code %d", *instance.ExitCode)
	}
	return "workload runtime exited unexpectedly"
}
