package readiness

import (
	"context"
	"time"
)

const (
	StateInactive = "inactive"
	StateWarming  = "warming"
	StateReady    = "ready"
	StateDegraded = "degraded"
	StateNotReady = "not_ready"
	StateStale    = "stale"
)

const (
	maxProbeEndpoints = 16
	maxEndpointLength = 2048
)

const (
	ReasonRuntimeInactive     = "runtime_inactive"
	ReasonWarmingUp           = "warming_up"
	ReasonReady               = "ready"
	ReasonListenerUnreachable = "listener_unreachable"
	ReasonGenerationMismatch  = "listener_generation_mismatch"
	ReasonProbeTimeout        = "probe_timeout"
	ReasonProbeStale          = "probe_stale"
	ReasonUnsupportedScheme   = "unsupported_endpoint_scheme"
	ReasonInvalidEndpoint     = "invalid_endpoint"
)

type Policy struct {
	Timeout          time.Duration
	Warmup           time.Duration
	SuccessThreshold int
	FailureThreshold int
	StaleAfter       time.Duration
}

type Observation struct {
	ServiceID         string
	WorkloadID        string
	Generation        int64
	Running           bool
	StartedAt         time.Time
	Endpoints         []string
	ExposureEndpoints []string
}

type EndpointStatus struct {
	Address       string
	Reachable     bool
	Reason        string
	LastCheckedAt time.Time
}

type Snapshot struct {
	ServiceID        string
	WorkloadID       string
	Generation       int64
	State            string
	Reason           string
	Ready            bool
	ExposureEligible bool
	LastProbeAt      time.Time
	Endpoints        []EndpointStatus
}

type CheckResult struct {
	Reachable bool
	Reason    string
}

type Prober interface {
	Check(context.Context, string, int64, time.Duration) CheckResult
}

func DefaultPolicy() Policy {
	return Policy{Timeout: time.Second, Warmup: 10 * time.Second, SuccessThreshold: 2, FailureThreshold: 3, StaleAfter: 5 * time.Second}
}
