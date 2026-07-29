package deployment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"
)

const (
	// PublicDirectResultVersion identifies the bounded ordinary MR-06 result.
	PublicDirectResultVersion = "ardents.topology.public-direct-result/v1"

	PublicDirectOutcomeReady            PublicDirectOutcome = "ready"
	PublicDirectOutcomeRecoveryRequired PublicDirectOutcome = "recovery_required"

	PublicDirectReasonPreflightUnavailable PublicDirectReason = "public_preflight_unavailable"
	PublicDirectReasonPreflightInvalid     PublicDirectReason = "public_preflight_invalid"
	PublicDirectReasonPreflightDenied      PublicDirectReason = "public_preflight_denied"
	PublicDirectReasonApplyFailed          PublicDirectReason = "host_apply_failed"
	PublicDirectReasonApplyInvalid         PublicDirectReason = "host_apply_invalid"
	PublicDirectReasonStatusUnavailable    PublicDirectReason = "topology_status_unavailable"
	PublicDirectReasonStatusDegraded       PublicDirectReason = "topology_status_degraded"

	PublicDirectApplyInstalled PublicDirectApplyAction = "installed"
	PublicDirectApplyUnchanged PublicDirectApplyAction = "unchanged"
	PublicDirectApplyRestarted PublicDirectApplyAction = "restarted"

	defaultPublicDirectStepTimeout  = 5 * time.Second
	maxPublicDirectStepTimeout      = 10 * time.Second
	maxPublicDirectOperationTime    = 30 * time.Second
	publicDirectPreflightMaxAge     = 2 * time.Minute
	publicDirectPreflightFutureSkew = 30 * time.Second
)

type PublicDirectOutcome string
type PublicDirectReason string
type PublicDirectApplyAction string

// PublicDirectResult is the bounded redacted public-direct reconciliation
// projection. Protected host and ingress material never appears here.
type PublicDirectResult struct {
	APIVersion         string              `json:"api_version"`
	Outcome            PublicDirectOutcome `json:"outcome"`
	PublicNodesReady   int                 `json:"public_nodes_ready"`
	OutboundNodesReady int                 `json:"outbound_nodes_ready"`
	RestartedNodes     int                 `json:"restarted_nodes"`
	Reason             PublicDirectReason  `json:"reason,omitempty"`
}

// PublicDirectPreflightTarget is protected operator-input validation scope.
// It does not carry or request reachability truth.
type PublicDirectPreflightTarget struct {
	ManifestDigest      string
	Slot                string
	SSHAlias            string
	HostKeyPinRef       string
	Address             string
	CertificateRef      string
	CertificateIdentity string
}

// PublicDirectPreflightObservation is protected exact pre-mutation evidence.
type PublicDirectPreflightObservation struct {
	ManifestDigest      string
	Slot                string
	Address             string
	CertificateRef      string
	CertificateIdentity string
	RouteReady          bool
	FirewallReady       bool
	CertificateReady    bool
	ObservedAt          time.Time
}

// PublicDirectHostTarget is protected host-local desired state.
type PublicDirectHostTarget struct {
	ManifestDigest      string
	ConfigurationDigest string
	Slot                string
	SSHAlias            string
	HostKeyPinRef       string
	Address             string
	CertificateRef      string
	CertificateIdentity string
	Plan                HostPlan
	StaticRecoveryPeers []string
	SignedDNSRoots      []string
}

// PublicDirectApplyObservation binds one idempotent apply outcome to exact
// desired and prior configuration digests.
type PublicDirectApplyObservation struct {
	ManifestDigest              string
	Slot                        string
	ConfigurationDigest         string
	PreviousConfigurationDigest string
	Action                      PublicDirectApplyAction
	IdentityPreserved           bool
}

// PublicDirectStatusTarget is protected exact status-observation scope.
type PublicDirectStatusTarget struct {
	ManifestDigest string
	Manifest       []byte
}

// PublicDirectStatusObservation binds the existing MR-02 status projection to
// the exact manifest being reconciled.
type PublicDirectStatusObservation struct {
	ManifestDigest string
	Status         TopologyStatus
}

type PublicDirectPreflight interface {
	Observe(context.Context, PublicDirectPreflightTarget) (PublicDirectPreflightObservation, error)
}

type PublicDirectHosts interface {
	Apply(context.Context, PublicDirectHostTarget) (PublicDirectApplyObservation, error)
}

type PublicDirectStatus interface {
	Observe(context.Context, PublicDirectStatusTarget) (PublicDirectStatusObservation, error)
}

// PublicDirectCoordinator reconciles operator-owned public ingress inputs
// around the runtime-owned AutoNAT publication gate.
type PublicDirectCoordinator struct {
	Preflight   PublicDirectPreflight
	Hosts       PublicDirectHosts
	Status      PublicDirectStatus
	StepTimeout time.Duration
	Now         func() time.Time
}

func (coordinator PublicDirectCoordinator) Reconcile(
	ctx context.Context,
	raw []byte,
) (PublicDirectResult, error) {
	if coordinator.Preflight == nil || coordinator.Hosts == nil || coordinator.Status == nil {
		return PublicDirectResult{}, ValidationError("public_direct_adapters_required")
	}
	timeout := coordinator.StepTimeout
	if timeout == 0 {
		timeout = defaultPublicDirectStepTimeout
	}
	if timeout < time.Millisecond || timeout > maxPublicDirectStepTimeout {
		return PublicDirectResult{}, ValidationError("public_direct_timeout_invalid")
	}
	manifest, err := decodeTopology(raw)
	if err != nil {
		return PublicDirectResult{}, err
	}
	if err := validateTopology(manifest); err != nil {
		return PublicDirectResult{}, err
	}
	if manifest.Mode != "public_direct" {
		return PublicDirectResult{}, ValidationError("topology_public_direct_required")
	}

	targets := publicDirectTargets(manifest, raw)
	operationContext, cancelOperation := context.WithTimeout(ctx, maxPublicDirectOperationTime)
	defer cancelOperation()
	result := PublicDirectResult{
		APIVersion: PublicDirectResultVersion,
		Outcome:    PublicDirectOutcomeRecoveryRequired,
	}

	// All public preflight checks finish before the first host mutation.
	type checkedPreflight struct {
		target      PublicDirectPreflightTarget
		observation PublicDirectPreflightObservation
	}
	preflights := make([]checkedPreflight, 0, len(targets))
	for _, target := range targets {
		if target.Plan.Ingress != "public_autonat_required" {
			continue
		}
		request := PublicDirectPreflightTarget{
			ManifestDigest: target.ManifestDigest,
			Slot:           target.Slot, SSHAlias: target.SSHAlias,
			HostKeyPinRef: target.HostKeyPinRef, Address: target.Address,
			CertificateRef:      target.CertificateRef,
			CertificateIdentity: target.CertificateIdentity,
		}
		step, cancel := context.WithTimeout(operationContext, timeout)
		observation, observeErr := coordinator.Preflight.Observe(step, request)
		cancel()
		if observeErr != nil {
			return publicDirectFailure(result, PublicDirectReasonPreflightUnavailable)
		}
		if !validPublicDirectPreflight(coordinator.nowUTC(), request, observation) {
			return publicDirectFailure(result, PublicDirectReasonPreflightInvalid)
		}
		if !observation.RouteReady || !observation.FirewallReady ||
			request.CertificateRef != "" && !observation.CertificateReady {
			return publicDirectFailure(result, PublicDirectReasonPreflightDenied)
		}
		preflights = append(preflights, checkedPreflight{
			target: request, observation: observation,
		})
	}
	// Sequential preflight can consume most of the evidence lifetime. Recheck
	// every observation against one instant immediately before the first apply.
	applyAt := coordinator.nowUTC()
	for _, checked := range preflights {
		if !validPublicDirectPreflight(
			applyAt,
			checked.target,
			checked.observation,
		) {
			return publicDirectFailure(result, PublicDirectReasonPreflightInvalid)
		}
	}

	for _, target := range targets {
		step, cancel := context.WithTimeout(operationContext, timeout)
		observation, applyErr := coordinator.Hosts.Apply(step, target)
		cancel()
		if applyErr != nil {
			return publicDirectFailure(result, PublicDirectReasonApplyFailed)
		}
		if !validPublicDirectApply(target, observation) {
			return publicDirectFailure(result, PublicDirectReasonApplyInvalid)
		}
		if observation.Action == PublicDirectApplyRestarted {
			result.RestartedNodes++
		}
	}

	statusTarget := PublicDirectStatusTarget{
		ManifestDigest: targets[0].ManifestDigest,
		Manifest:       append([]byte(nil), raw...),
	}
	step, cancel := context.WithTimeout(operationContext, timeout)
	observation, observeErr := coordinator.Status.Observe(step, statusTarget)
	cancel()
	if observeErr != nil {
		return publicDirectFailure(result, PublicDirectReasonStatusUnavailable)
	}
	publicReady, outboundReady, valid := validPublicDirectStatus(
		observation,
		targets,
	)
	if !valid {
		return publicDirectFailure(result, PublicDirectReasonStatusDegraded)
	}
	result.Outcome = PublicDirectOutcomeReady
	result.PublicNodesReady = publicReady
	result.OutboundNodesReady = outboundReady
	return result, nil
}

func publicDirectTargets(
	manifest topologyManifest,
	raw []byte,
) []PublicDirectHostTarget {
	manifestHash := sha256.Sum256(raw)
	manifestDigest := hex.EncodeToString(manifestHash[:])
	plan := compilePlan(manifest)
	plans := make(map[string]HostPlan, len(plan.Hosts))
	for _, host := range plan.Hosts {
		plans[host.Slot] = host
	}
	targets := make([]PublicDirectHostTarget, 0, len(manifest.Nodes))
	for _, node := range manifest.Nodes {
		target := PublicDirectHostTarget{
			ManifestDigest: manifestDigest,
			Slot:           node.Slot, SSHAlias: node.Host.SSHAlias,
			HostKeyPinRef:       node.Host.HostKeyPinRef,
			Plan:                plans[node.Slot],
			StaticRecoveryPeers: append([]string(nil), node.StaticRecoveryPeers...),
			SignedDNSRoots:      append([]string(nil), manifest.SignedDNSRoots...),
		}
		if node.Ingress.Address != nil {
			target.Address = *node.Ingress.Address
		}
		if node.Ingress.CertificateRef != nil {
			target.CertificateRef = *node.Ingress.CertificateRef
		}
		if node.Ingress.CertificateIdentity != nil {
			target.CertificateIdentity = *node.Ingress.CertificateIdentity
		}
		sort.Strings(target.StaticRecoveryPeers)
		sort.Strings(target.SignedDNSRoots)
		target.ConfigurationDigest = publicDirectConfigurationDigest(target)
		targets = append(targets, target)
	}
	sort.Slice(targets, func(left, right int) bool {
		return targets[left].Slot < targets[right].Slot
	})
	return targets
}

func publicDirectConfigurationDigest(target PublicDirectHostTarget) string {
	// Deliberately exclude ManifestDigest and protected access references:
	// unrelated manifest edits must not force a restart on every host.
	value := struct {
		Plan                HostPlan `json:"plan"`
		Address             string   `json:"address"`
		CertificateRef      string   `json:"certificate_ref"`
		CertificateIdentity string   `json:"certificate_identity"`
		StaticRecoveryPeers []string `json:"static_recovery_peers"`
		SignedDNSRoots      []string `json:"signed_dns_roots"`
	}{
		Plan: target.Plan, Address: target.Address,
		CertificateRef:      target.CertificateRef,
		CertificateIdentity: target.CertificateIdentity,
		StaticRecoveryPeers: target.StaticRecoveryPeers,
		SignedDNSRoots:      target.SignedDNSRoots,
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		panic("public-direct configuration digest uses only JSON-safe values")
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func (coordinator PublicDirectCoordinator) nowUTC() time.Time {
	if coordinator.Now != nil {
		return coordinator.Now().UTC()
	}
	return time.Now().UTC()
}

func validPublicDirectPreflight(
	now time.Time,
	target PublicDirectPreflightTarget,
	observation PublicDirectPreflightObservation,
) bool {
	observedAt := observation.ObservedAt.UTC()
	if observation.ManifestDigest != target.ManifestDigest ||
		observation.Slot != target.Slot ||
		observation.Address != target.Address ||
		observation.CertificateRef != target.CertificateRef ||
		observation.CertificateIdentity != target.CertificateIdentity ||
		observedAt.IsZero() ||
		observedAt.Before(now.Add(-publicDirectPreflightMaxAge)) ||
		observedAt.After(now.Add(publicDirectPreflightFutureSkew)) {
		return false
	}
	// TCP preflight cannot introduce a certificate claim.
	return target.CertificateRef != "" || !observation.CertificateReady
}

func validPublicDirectApply(
	target PublicDirectHostTarget,
	observation PublicDirectApplyObservation,
) bool {
	if observation.ManifestDigest != target.ManifestDigest ||
		observation.Slot != target.Slot ||
		observation.ConfigurationDigest != target.ConfigurationDigest ||
		!observation.IdentityPreserved {
		return false
	}
	previous := observation.PreviousConfigurationDigest
	switch observation.Action {
	case PublicDirectApplyInstalled:
		return previous == ""
	case PublicDirectApplyUnchanged:
		return previous == target.ConfigurationDigest
	case PublicDirectApplyRestarted:
		return previous != "" && previous != target.ConfigurationDigest &&
			validSHA256Digest(previous)
	default:
		return false
	}
}

func validSHA256Digest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && hex.EncodeToString(decoded) == value
}

func validPublicDirectStatus(
	observation PublicDirectStatusObservation,
	targets []PublicDirectHostTarget,
) (int, int, bool) {
	status := observation.Status
	if len(targets) != exactTopologyNodeCount ||
		observation.ManifestDigest != targets[0].ManifestDigest ||
		status.APIVersion != TopologyStatusVersion ||
		status.Outcome != TopologyOutcomeReady ||
		len(status.Nodes) != exactTopologyNodeCount {
		return 0, 0, false
	}
	publicReady := 0
	outboundReady := 0
	for index, node := range status.Nodes {
		target := targets[index]
		if node.Slot != target.Slot || node.Role != target.Plan.Role ||
			!node.Ready || node.Observation != NodeObservationComplete ||
			node.Readiness != NodeTruthReady || !node.Joined ||
			node.Reachability != NodeTruthReady ||
			node.Image != NodeImageMatch {
			return 0, 0, false
		}
		if target.Plan.PersistentStore {
			if node.Store != NodeTruthReady {
				return 0, 0, false
			}
		} else if node.Store != NodeTruthNotRequired {
			return 0, 0, false
		}
		switch target.Plan.Ingress {
		case "public_autonat_required":
			publicReady++
		case "outbound_only":
			outboundReady++
		default:
			return 0, 0, false
		}
	}
	return publicReady, outboundReady,
		publicReady >= 2 && publicReady+outboundReady == exactTopologyNodeCount
}

func publicDirectFailure(
	result PublicDirectResult,
	reason PublicDirectReason,
) (PublicDirectResult, error) {
	result.Outcome = PublicDirectOutcomeRecoveryRequired
	result.Reason = reason
	return result, errors.New(string(reason))
}
