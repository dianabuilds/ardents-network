package execution

import (
	"ardents/internal/workload/registry"
	"fmt"
	"time"
)

func requestFromSpec(spec registry.Spec) Request {
	ingress := make([]IngressRequest, 0, len(spec.Services))
	for _, service := range spec.Services {
		ingress = append(ingress, IngressRequest{
			Mode:           service.Mode,
			Endpoints:      append([]string(nil), service.Endpoints...),
			ProbeEndpoints: append([]string(nil), service.ProbeEndpoints...),
		})
	}
	return Request{
		WorkloadID: spec.ID,
		Config:     spec.Config,
		PolicyRef:  spec.PolicyRef,
		Ingress:    ingress,
	}
}

func RejectAdmission(item Status, now time.Time, err error) Status {
	item.Observed = ObservedFailed
	item.Reason = "admission rejected: " + err.Error()
	item.NeedsOperatorAction = true
	item.LastTransitionAt = now
	item.PublishedServices = ServiceStatuses(item.Spec, false, item.Reason)
	return item
}

func ReconcilePresent(item Status, now time.Time) Status {
	item.Observed = ObservedAccepted
	item.Reason = ""
	item.NeedsOperatorAction = false
	item.Instance = Instance{}
	item.LastTransitionAt = now
	item.PublishedServices = ServiceStatuses(item.Spec, false, "workload not running")
	return item
}

func MarkRunning(item Status, now time.Time) Status {
	item.Observed = ObservedRunning
	item.Reason = ""
	item.NeedsOperatorAction = false
	item.LastTransitionAt = now
	item.PublishedServices = ServiceStatuses(item.Spec, true, "")
	return item
}

func DegradedRunningAgainstDesired(item Status, instance Instance, now time.Time) (Status, bool) {
	if item.Spec.Desired != registry.DesiredStopped && item.Spec.Desired != registry.DesiredDisabled {
		return item, false
	}
	item.Instance = instance
	item.Observed = ObservedDegraded
	item.Reason = "workload is still running after stop request"
	item.NeedsOperatorAction = true
	item.LastTransitionAt = now
	item.PublishedServices = ServiceStatuses(item.Spec, false, item.Reason)
	return item, true
}

func (s *Service) observedAfterUnexpectedExit(item Status, now time.Time) Status {
	item.Instance.Running = false
	item.Instance.PID = 0
	if item.Instance.FinishedAt.IsZero() {
		item.Instance.FinishedAt = now
	}
	if item.Instance.Reason == "" {
		item.Instance.Reason = "process exited before observation sync"
	}
	switch item.Spec.Desired {
	case registry.DesiredRunning:
		item.RestartCount++
		item.Reason = unexpectedExitReason(item.Instance)
		terminal := item.Instance.OOMKilled || item.Spec.RestartPolicy == "never" || item.RestartCount > s.restartBudget
		if terminal {
			item.Observed = ObservedFailed
			item.NeedsOperatorAction = true
		} else {
			item.Observed = ObservedDegraded
			item.NeedsOperatorAction = false
			item.Reason += "; restart reconciliation pending"
		}
		item.PublishedServices = ServiceStatuses(item.Spec, false, item.Reason)
	case registry.DesiredPresent:
		item.Observed = ObservedAccepted
		item.Reason = ""
		item.NeedsOperatorAction = false
		item.PublishedServices = ServiceStatuses(item.Spec, false, "workload not running")
	default:
		item.Observed = ObservedStopped
		item.Reason = ""
		item.NeedsOperatorAction = false
		item.PublishedServices = ServiceStatuses(item.Spec, false, "workload not running")
	}
	item.LastTransitionAt = now
	return item
}

func unexpectedExitReason(instance Instance) string {
	if instance.OOMKilled {
		return "workload exceeded its memory limit"
	}
	if instance.ExitCode != nil {
		return fmt.Sprintf("workload exited with code %d", *instance.ExitCode)
	}
	return "workload runtime exited unexpectedly"
}
