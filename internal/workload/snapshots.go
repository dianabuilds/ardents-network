package workload

import (
	"time"

	"ardents/internal/workload/registry"
)

type PublishedServiceSnapshot struct {
	ID             string   `json:"id,omitempty"`
	Type           string   `json:"type,omitempty"`
	Owner          string   `json:"owner,omitempty"`
	Mode           string   `json:"mode,omitempty"`
	Published      bool     `json:"published,omitempty"`
	Endpoints      []string `json:"endpoints,omitempty"`
	ProbeEndpoints []string `json:"probe_endpoints,omitempty"`
	Reason         string   `json:"reason,omitempty"`
}

type SpecSnapshot struct {
	ID            string                         `json:"id,omitempty"`
	Kind          string                         `json:"kind,omitempty"`
	Owner         string                         `json:"owner,omitempty"`
	Config        string                         `json:"config,omitempty"`
	Desired       string                         `json:"desired,omitempty"`
	Services      []PublishedServiceSnapshot     `json:"services,omitempty"`
	Requirements  []registry.WorkloadRequirement `json:"requirements,omitempty"`
	PolicyRef     string                         `json:"policy_ref,omitempty"`
	RestartPolicy string                         `json:"restart_policy,omitempty"`
}

type InstanceSnapshot struct {
	WorkloadID       string    `json:"workload_id,omitempty"`
	Generation       int64     `json:"generation,omitempty"`
	Running          bool      `json:"running,omitempty"`
	StartedAt        time.Time `json:"started_at"`
	FinishedAt       time.Time `json:"finished_at"`
	ExitCode         *int      `json:"exit_code,omitempty"`
	OOMKilled        bool      `json:"oom_killed,omitempty"`
	Restarts         int       `json:"restarts,omitempty"`
	MemoryLimitBytes int64     `json:"memory_limit_bytes,omitempty"`
	NanoCPUs         int64     `json:"nano_cpus,omitempty"`
	PIDsLimit        int64     `json:"pids_limit,omitempty"`
	Reason           string    `json:"reason,omitempty"`
}

type StatusSnapshot struct {
	Spec                SpecSnapshot               `json:"spec"`
	Observed            string                     `json:"observed,omitempty"`
	Reason              string                     `json:"reason,omitempty"`
	LastTransitionAt    time.Time                  `json:"last_transition_at"`
	NeedsOperatorAction bool                       `json:"needs_operator_action,omitempty"`
	RestartCount        int                        `json:"restart_count,omitempty"`
	PublishedServices   []PublishedServiceSnapshot `json:"published_services,omitempty"`
	Instance            InstanceSnapshot           `json:"instance"`
	ObservedAt          time.Time                  `json:"observed_at"`
	ObservationDegraded bool                       `json:"observation_degraded,omitempty"`
}

type StateSnapshot struct {
	State   string `json:"state,omitempty"`
	Desired int    `json:"desired,omitempty"`
	Active  int    `json:"active,omitempty"`
}
