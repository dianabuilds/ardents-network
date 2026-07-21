// Package readiness owns hosted-service probe execution and readiness truth.
// It does not own workload lifecycle or publication.
package readiness

import (
	"context"
	"reflect"
	"sync"
	"time"
)

type Controller struct {
	mu     sync.Mutex
	policy Policy
	prober Prober
	items  map[string]probeState
}

type probeState struct {
	observation Observation
	snapshot    Snapshot
	successes   int
	failures    int
	everReady   bool
}

func NewController(policy Policy) *Controller {
	return NewControllerWithProber(policy, NetworkProber{})
}

func NewControllerWithProber(policy Policy, prober Prober) *Controller {
	policy = normalizePolicy(policy)
	if prober == nil {
		prober = NetworkProber{}
	}
	return &Controller{policy: policy, prober: prober, items: map[string]probeState{}}
}

func (c *Controller) Observe(ctx context.Context, observation Observation, now time.Time) Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()

	state := c.items[observation.ServiceID]
	if generationRegressed(state.observation, observation) {
		state.snapshot = baseSnapshot(observation, nil, now)
		state.snapshot.State = StateNotReady
		state.snapshot.Reason = ReasonGenerationMismatch
		c.items[observation.ServiceID] = state
		return cloneSnapshot(state.snapshot)
	}
	if identityChanged(state.observation, observation) {
		state = probeState{observation: cloneObservation(observation)}
	} else {
		state.observation = cloneObservation(observation)
	}
	if !observation.Running {
		state.snapshot = inactiveSnapshot(observation, now)
		c.items[observation.ServiceID] = state
		return cloneSnapshot(state.snapshot)
	}
	state = c.runChecks(ctx, state, now)
	c.items[observation.ServiceID] = state
	return cloneSnapshot(state.snapshot)
}

func generationRegressed(previous, current Observation) bool {
	return previous.ServiceID != "" && previous.WorkloadID == current.WorkloadID && current.Generation < previous.Generation
}

func (c *Controller) Snapshot(serviceID string, now time.Time) (Snapshot, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	state, ok := c.items[serviceID]
	if !ok {
		return Snapshot{}, false
	}
	snapshot := cloneSnapshot(state.snapshot)
	if snapshot.Ready && now.Sub(snapshot.LastProbeAt) > c.policy.StaleAfter {
		snapshot.State = StateStale
		snapshot.Reason = ReasonProbeStale
		snapshot.Ready = false
		snapshot.ExposureEligible = false
	}
	return snapshot, true
}

func (c *Controller) runChecks(ctx context.Context, state probeState, now time.Time) probeState {
	if !validEndpointSet(state.observation.Endpoints) {
		state.snapshot = baseSnapshot(state.observation, nil, now)
		return c.recordFailure(state, ReasonInvalidEndpoint, now)
	}
	results := make([]EndpointStatus, 0, len(state.observation.Endpoints))
	failureReason := ""
	for _, endpoint := range state.observation.Endpoints {
		result := c.prober.Check(ctx, endpoint, state.observation.Generation, c.policy.Timeout)
		results = append(results, EndpointStatus{Address: endpoint, Reachable: result.Reachable, Reason: result.Reason, LastCheckedAt: now})
		if !result.Reachable && failureReason == "" {
			failureReason = result.Reason
		}
	}
	if len(results) == 0 && failureReason == "" {
		failureReason = ReasonInvalidEndpoint
	}
	state.snapshot = baseSnapshot(state.observation, results, now)
	if failureReason == "" {
		return c.recordSuccess(state)
	}
	return c.recordFailure(state, failureReason, now)
}

func validEndpointSet(endpoints []string) bool {
	if len(endpoints) == 0 || len(endpoints) > maxProbeEndpoints {
		return false
	}
	for _, endpoint := range endpoints {
		if endpoint == "" || len(endpoint) > maxEndpointLength {
			return false
		}
	}
	return true
}

func (c *Controller) recordSuccess(state probeState) probeState {
	state.successes++
	state.failures = 0
	state.snapshot.State = StateWarming
	state.snapshot.Reason = ReasonWarmingUp
	if state.successes >= c.policy.SuccessThreshold {
		state.everReady = true
		state.snapshot.State = StateReady
		state.snapshot.Reason = ReasonReady
		state.snapshot.Ready = true
		state.snapshot.ExposureEligible = true
	}
	return state
}

func (c *Controller) recordFailure(state probeState, reason string, now time.Time) probeState {
	state.successes = 0
	state.failures++
	state.snapshot.Reason = reason
	if now.Sub(state.observation.StartedAt) < c.policy.Warmup {
		state.snapshot.State = StateWarming
		state.snapshot.Reason = ReasonWarmingUp
		return state
	}
	if state.everReady && state.failures < c.policy.FailureThreshold {
		state.snapshot.State = StateDegraded
		state.snapshot.Ready = true
		state.snapshot.ExposureEligible = true
		return state
	}
	state.snapshot.State = StateNotReady
	return state
}

func normalizePolicy(policy Policy) Policy {
	defaults := DefaultPolicy()
	if policy.Timeout <= 0 {
		policy.Timeout = defaults.Timeout
	}
	if policy.Warmup < 0 {
		policy.Warmup = defaults.Warmup
	}
	if policy.SuccessThreshold <= 0 {
		policy.SuccessThreshold = defaults.SuccessThreshold
	}
	if policy.FailureThreshold <= 0 {
		policy.FailureThreshold = defaults.FailureThreshold
	}
	if policy.StaleAfter <= 0 {
		policy.StaleAfter = defaults.StaleAfter
	}
	return policy
}

func identityChanged(previous, current Observation) bool {
	return previous.ServiceID == "" || previous.WorkloadID != current.WorkloadID || previous.Generation != current.Generation ||
		!reflect.DeepEqual(previous.Endpoints, current.Endpoints) || !reflect.DeepEqual(previous.ExposureEndpoints, current.ExposureEndpoints)
}

func baseSnapshot(observation Observation, endpoints []EndpointStatus, now time.Time) Snapshot {
	return Snapshot{ServiceID: observation.ServiceID, WorkloadID: observation.WorkloadID, Generation: observation.Generation, LastProbeAt: now, Endpoints: endpoints}
}

func inactiveSnapshot(observation Observation, now time.Time) Snapshot {
	snapshot := baseSnapshot(observation, nil, now)
	snapshot.State = StateInactive
	snapshot.Reason = ReasonRuntimeInactive
	return snapshot
}

func cloneObservation(in Observation) Observation {
	in.Endpoints = append([]string(nil), in.Endpoints...)
	in.ExposureEndpoints = append([]string(nil), in.ExposureEndpoints...)
	return in
}

func cloneSnapshot(in Snapshot) Snapshot {
	in.Endpoints = append([]EndpointStatus(nil), in.Endpoints...)
	return in
}
