package deployment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"time"
)

const (
	// PrivateLANResultVersion identifies the bounded ordinary MR-05 result.
	PrivateLANResultVersion = "ardents.topology.private-lan-result/v1"

	PrivateLANOutcomeReady            PrivateLANOutcome = "ready"
	PrivateLANOutcomeRecoveryRequired PrivateLANOutcome = "recovery_required"

	PrivateLANReasonHostApplyFailed   PrivateLANReason = "host_apply_failed"
	PrivateLANReasonProbeFailed       PrivateLANReason = "cross_host_probe_failed"
	PrivateLANReasonProbeInvalid      PrivateLANReason = "cross_host_probe_invalid"
	PrivateLANReasonProofApplyFailed  PrivateLANReason = "probe_proof_apply_failed"
	PrivateLANReasonStatusUnavailable PrivateLANReason = "topology_status_unavailable"
	PrivateLANReasonStatusDegraded    PrivateLANReason = "topology_status_degraded"

	defaultPrivateLANStepTimeout = 5 * time.Second
	maxPrivateLANStepTimeout     = 10 * time.Second
	maxPrivateLANOperationTime   = 30 * time.Second
	maxPrivateLANStoreCount      = 1_000_000
	privateLANProbeMaxAge        = 2 * time.Minute
	privateLANProbeFutureSkew    = 30 * time.Second
)

type PrivateLANOutcome string
type PrivateLANReason string

// PrivateLANResult is the redacted formation/reconciliation projection.
type PrivateLANResult struct {
	APIVersion           string            `json:"api_version"`
	Outcome              PrivateLANOutcome `json:"outcome"`
	NodesReady           int               `json:"nodes_ready"`
	RetainedStoreFetches int               `json:"retained_store_fetches"`
	StoreGaps            int               `json:"store_gaps"`
	Reason               PrivateLANReason  `json:"reason,omitempty"`
}

// PrivateLANHostTarget is protected deployment input and must never be copied
// into PrivateLANResult.
type PrivateLANHostTarget struct {
	ManifestDigest      string
	Slot                string
	SSHAlias            string
	HostKeyPinRef       string
	Address             string
	Plan                HostPlan
	StaticRecoveryPeers []string
	SignedDNSRoots      []string
}

// PrivateLANProbeTarget is the protected different-host dial request.
type PrivateLANProbeTarget struct {
	ManifestDigest string
	SourceSlot     string
	SourceSSHAlias string
	TargetSlot     string
	Address        string
}

// PrivateLANProof is protected deployment evidence passed to the host-local
// adapter. The adapter may translate it to the runtime's proof type.
type PrivateLANProof struct {
	SourceSlot string
	TargetSlot string
	Address    string
	ObservedAt time.Time
	Success    bool
}

// PrivateLANObservation combines the existing bounded status projection with
// explicit bounded Store cache outcomes.
type PrivateLANObservation struct {
	Status               TopologyStatus
	RetainedStoreFetches int
	StoreGaps            int
}

type PrivateLANHosts interface {
	Apply(context.Context, PrivateLANHostTarget) error
	ApplyProbe(context.Context, PrivateLANHostTarget, PrivateLANProof) error
}

type PrivateLANDialer interface {
	Probe(context.Context, PrivateLANProbeTarget) (time.Time, error)
}

type PrivateLANStatus interface {
	Observe(context.Context, []byte) (PrivateLANObservation, error)
}

// PrivateLANCoordinator deterministically forms or reconciles the accepted
// three-host private-LAN topology through consumer-owned host boundaries.
type PrivateLANCoordinator struct {
	Hosts       PrivateLANHosts
	Dialer      PrivateLANDialer
	Status      PrivateLANStatus
	StepTimeout time.Duration
	Now         func() time.Time
}

func (coordinator PrivateLANCoordinator) Reconcile(
	ctx context.Context,
	raw []byte,
) (PrivateLANResult, error) {
	if coordinator.Hosts == nil || coordinator.Dialer == nil || coordinator.Status == nil {
		return PrivateLANResult{}, ValidationError("private_lan_adapters_required")
	}
	timeout := coordinator.StepTimeout
	if timeout == 0 {
		timeout = defaultPrivateLANStepTimeout
	}
	if timeout < time.Millisecond || timeout > maxPrivateLANStepTimeout {
		return PrivateLANResult{}, ValidationError("private_lan_timeout_invalid")
	}
	manifest, err := decodeTopology(raw)
	if err != nil {
		return PrivateLANResult{}, err
	}
	if err := validateTopology(manifest); err != nil {
		return PrivateLANResult{}, err
	}
	if manifest.Mode != "private_lan" {
		return PrivateLANResult{}, ValidationError("topology_private_lan_required")
	}
	targets := privateLANTargets(manifest, raw)
	operationContext, cancelOperation := context.WithTimeout(ctx, maxPrivateLANOperationTime)
	defer cancelOperation()
	result := PrivateLANResult{
		APIVersion: PrivateLANResultVersion,
		Outcome:    PrivateLANOutcomeRecoveryRequired,
	}
	for _, target := range targets {
		step, cancel := context.WithTimeout(operationContext, timeout)
		applyErr := coordinator.Hosts.Apply(step, target)
		cancel()
		if applyErr != nil {
			return privateLANFailure(result, PrivateLANReasonHostApplyFailed)
		}
	}
	now := time.Now().UTC()
	if coordinator.Now != nil {
		now = coordinator.Now().UTC()
	}
	for index, target := range targets {
		source := targets[(index+1)%len(targets)]
		probeTarget := PrivateLANProbeTarget{
			ManifestDigest: target.ManifestDigest,
			SourceSlot:     source.Slot, SourceSSHAlias: source.SSHAlias,
			TargetSlot: target.Slot, Address: target.Address,
		}
		step, cancel := context.WithTimeout(operationContext, timeout)
		observedAt, probeErr := coordinator.Dialer.Probe(step, probeTarget)
		cancel()
		probe := PrivateLANProof{
			SourceSlot: source.Slot, TargetSlot: target.Slot,
			Address: target.Address, ObservedAt: observedAt.UTC(),
			Success: probeErr == nil,
		}
		if probeErr != nil {
			probe.ObservedAt = now
			withdraw, withdrawCancel := context.WithTimeout(operationContext, timeout)
			_ = coordinator.Hosts.ApplyProbe(withdraw, target, probe)
			withdrawCancel()
			return privateLANFailure(result, PrivateLANReasonProbeFailed)
		}
		if !validPrivateLANProbeTime(now, observedAt) {
			return privateLANFailure(result, PrivateLANReasonProbeInvalid)
		}
		step, cancel = context.WithTimeout(operationContext, timeout)
		applyErr := coordinator.Hosts.ApplyProbe(step, target, probe)
		cancel()
		if applyErr != nil {
			return privateLANFailure(result, PrivateLANReasonProofApplyFailed)
		}
	}
	step, cancel := context.WithTimeout(operationContext, timeout)
	observation, observeErr := coordinator.Status.Observe(step, append([]byte(nil), raw...))
	cancel()
	if observeErr != nil {
		return privateLANFailure(result, PrivateLANReasonStatusUnavailable)
	}
	if !validPrivateLANObservation(observation, targets) {
		return privateLANFailure(result, PrivateLANReasonStatusDegraded)
	}
	result.Outcome = PrivateLANOutcomeReady
	result.NodesReady = len(observation.Status.Nodes)
	result.RetainedStoreFetches = observation.RetainedStoreFetches
	result.StoreGaps = observation.StoreGaps
	return result, nil
}

func privateLANTargets(manifest topologyManifest, raw []byte) []PrivateLANHostTarget {
	digest := sha256.Sum256(raw)
	manifestDigest := hex.EncodeToString(digest[:])
	plan := compilePlan(manifest)
	plans := make(map[string]HostPlan, len(plan.Hosts))
	for _, host := range plan.Hosts {
		plans[host.Slot] = host
	}
	targets := make([]PrivateLANHostTarget, 0, len(manifest.Nodes))
	for _, node := range manifest.Nodes {
		targets = append(targets, PrivateLANHostTarget{
			ManifestDigest: manifestDigest,
			Slot:           node.Slot, SSHAlias: node.Host.SSHAlias,
			HostKeyPinRef: node.Host.HostKeyPinRef,
			Address:       *node.Ingress.Address, Plan: plans[node.Slot],
			StaticRecoveryPeers: append([]string(nil), node.StaticRecoveryPeers...),
			SignedDNSRoots:      append([]string(nil), manifest.SignedDNSRoots...),
		})
	}
	sort.Slice(targets, func(left, right int) bool {
		return targets[left].Slot < targets[right].Slot
	})
	for index := range targets {
		sort.Strings(targets[index].StaticRecoveryPeers)
		sort.Strings(targets[index].SignedDNSRoots)
	}
	return targets
}

func validPrivateLANProbeTime(now, observedAt time.Time) bool {
	observedAt = observedAt.UTC()
	return !observedAt.IsZero() &&
		!observedAt.Before(now.Add(-privateLANProbeMaxAge)) &&
		!observedAt.After(now.Add(privateLANProbeFutureSkew))
}

func validPrivateLANObservation(
	observation PrivateLANObservation,
	targets []PrivateLANHostTarget,
) bool {
	if observation.Status.APIVersion != TopologyStatusVersion ||
		observation.Status.Outcome != TopologyOutcomeReady ||
		len(observation.Status.Nodes) != exactTopologyNodeCount ||
		len(targets) != exactTopologyNodeCount ||
		observation.RetainedStoreFetches < 0 ||
		observation.RetainedStoreFetches > maxPrivateLANStoreCount ||
		observation.StoreGaps < 0 ||
		observation.StoreGaps > maxPrivateLANStoreCount {
		return false
	}
	for index, node := range observation.Status.Nodes {
		if node.Slot != targets[index].Slot || !node.Ready ||
			node.Observation != NodeObservationComplete ||
			node.Readiness != NodeTruthReady || !node.Joined ||
			node.Reachability != NodeTruthReady ||
			node.Image != NodeImageMatch {
			return false
		}
		if targets[index].Plan.PersistentStore && node.Store != NodeTruthReady {
			return false
		}
	}
	return true
}

func privateLANFailure(
	result PrivateLANResult,
	reason PrivateLANReason,
) (PrivateLANResult, error) {
	result.Outcome = PrivateLANOutcomeRecoveryRequired
	result.Reason = reason
	return result, errors.New(string(reason))
}
