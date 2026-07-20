package observedstate

import (
	"time"

	hostingexposure "ardents/internal/hosting/exposure"
	"ardents/internal/workload/execution"
	domainworkload "ardents/internal/workload/workload"
)

const (
	Accepted  = "accepted"
	Preparing = "preparing"
	Running   = "running"
	Stopping  = "stopping"
	Stopped   = "stopped"
	Failed    = "failed"
	Degraded  = "degraded"
	Removed   = "removed"
)

type PublishedServiceStatus = hostingexposure.PublishedStatus

type Status struct {
	Spec                domainworkload.Spec      `json:"spec"`
	Observed            string                   `json:"observed"`
	Reason              string                   `json:"reason,omitempty"`
	LastTransitionAt    time.Time                `json:"last_transition_at,omitempty"`
	NeedsOperatorAction bool                     `json:"needs_operator_action"`
	RestartCount        int                      `json:"restart_count"`
	PublishedServices   []PublishedServiceStatus `json:"published_services,omitempty"`
	Instance            execution.Instance       `json:"instance"`
}
