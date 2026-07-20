package execution

import (
	"context"
	"time"
)

type Request struct {
	WorkloadID string
	Config     string
	PolicyRef  string
	Ingress    []IngressRequest
}

type IngressRequest struct {
	Mode           string
	Endpoints      []string
	ProbeEndpoints []string
}

type IngressBinding struct {
	Port        uint16 `json:"port"`
	ProbeHost   string `json:"probe_host"`
	BindAddress string `json:"bind_address"`
}

type PreparedWorkload struct {
	WorkloadID string           `json:"workload_id"`
	Generation int64            `json:"generation"`
	PreparedAt time.Time        `json:"prepared_at"`
	Handle     string           `json:"handle,omitempty"`
	PolicyRef  string           `json:"policy_ref,omitempty"`
	Ingress    []IngressBinding `json:"ingress,omitempty"`
}

type Instance struct {
	WorkloadID       string    `json:"workload_id"`
	Generation       int64     `json:"generation"`
	RuntimeID        string    `json:"runtime_id,omitempty"`
	Running          bool      `json:"running"`
	PID              int       `json:"pid,omitempty"`
	StartedAt        time.Time `json:"started_at,omitempty"`
	FinishedAt       time.Time `json:"finished_at,omitempty"`
	ExitCode         *int      `json:"exit_code,omitempty"`
	OOMKilled        bool      `json:"oom_killed,omitempty"`
	Restarts         int       `json:"restarts,omitempty"`
	MemoryLimitBytes int64     `json:"memory_limit_bytes,omitempty"`
	NanoCPUs         int64     `json:"nano_cpus,omitempty"`
	PIDsLimit        int64     `json:"pids_limit,omitempty"`
	Runtime          string    `json:"runtime,omitempty"`
	TrustClass       string    `json:"trust_class,omitempty"`
	Reason           string    `json:"reason,omitempty"`
}

type Executor interface {
	Prepare(context.Context, Request) (PreparedWorkload, error)
	Start(context.Context, PreparedWorkload) (Instance, error)
	Stop(context.Context, Instance) error
	Inspect(context.Context, string) (Instance, error)
}

type Remover interface {
	Remove(context.Context, Instance) error
}

type Inventory interface {
	Managed(context.Context) ([]Instance, error)
}

type AncillaryReconciler interface {
	ReconcileAncillary(context.Context, []Instance) error
}
