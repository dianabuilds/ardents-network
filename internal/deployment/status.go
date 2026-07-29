package deployment

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	identityprincipal "ardents/internal/identity/principal"
)

const (
	// TopologyStatusVersion identifies the bounded ordinary MR-02 projection.
	TopologyStatusVersion = "ardents.topology.status/v1"

	TopologyOutcomeReady    TopologyOutcome = "ready"
	TopologyOutcomeDegraded TopologyOutcome = "degraded"
	TopologyOutcomePartial  TopologyOutcome = "partial"

	NodeObservationComplete    NodeObservationState = "complete"
	NodeObservationUnavailable NodeObservationState = "unavailable"

	NodeTruthReady       NodeTruth = "ready"
	NodeTruthDegraded    NodeTruth = "degraded"
	NodeTruthUnavailable NodeTruth = "unavailable"
	NodeTruthNotRequired NodeTruth = "not_required"

	NodeImageMatch      NodeImageState = "match"
	NodeImageMismatch   NodeImageState = "mismatch"
	NodeImageUnverified NodeImageState = "unverified"

	StatusReasonNodeIdentityMismatch          StatusReason = "node_identity_mismatch"
	StatusReasonImageUnverified               StatusReason = "image_unverified"
	StatusReasonImageMismatch                 StatusReason = "image_mismatch"
	StatusReasonCompositeReadinessDegraded    StatusReason = "composite_readiness_degraded"
	StatusReasonNetworkNotJoined              StatusReason = "network_not_joined"
	StatusReasonPublicReachabilityUnverified  StatusReason = "public_reachability_unverified"
	StatusReasonPrivateReachabilityUnverified StatusReason = "private_reachability_unverified"
	StatusReasonOutboundReachabilityMismatch  StatusReason = "outbound_reachability_mismatch"
	StatusReasonPersistentStoreUnavailable    StatusReason = "persistent_store_unavailable"
	StatusReasonPersistentStoreDegraded       StatusReason = "persistent_store_degraded"

	defaultNodeStatusTimeout = 10 * time.Second
	maxTopologyStatusTimeout = 30 * time.Second
	maxObservationString     = 512
)

// TopologyOutcome is the closed aggregate health vocabulary.
type TopologyOutcome string

// NodeObservationState is the closed observation-completeness vocabulary.
type NodeObservationState string

// NodeTruth is the closed readiness, reachability, and Store vocabulary.
type NodeTruth string

// NodeImageState is the closed expected-image comparison vocabulary.
type NodeImageState string

// StatusReason is the closed redacted reason vocabulary.
type StatusReason string

// ProbeErrorCode is one closed local/remote failure class owned by MR-02.
type ProbeErrorCode string

const (
	ProbeHostKeyMismatch        ProbeErrorCode = "host_key_mismatch"
	ProbeTunnelTimeout          ProbeErrorCode = "tunnel_timeout"
	ProbeTunnelFailure          ProbeErrorCode = "tunnel_failure"
	ProbeLocalSignerUnavailable ProbeErrorCode = "local_signer_unavailable"
	ProbeRemoteUnauthenticated  ProbeErrorCode = "remote_unauthenticated"
	ProbeRemoteDenied           ProbeErrorCode = "remote_denied"
	ProbeNodeUnavailable        ProbeErrorCode = "node_unavailable"
	ProbeRemoteInvalidResponse  ProbeErrorCode = "remote_invalid_response"
)

// ProbeError carries only a stable redacted failure code.
type ProbeError ProbeErrorCode

func (err ProbeError) Error() string { return string(err) }

// NodeStatusProbe is deployment's bounded host-access boundary for status.
type NodeStatusProbe interface {
	Observe(context.Context, NodeStatusTarget) (NodeObservation, error)
}

// NodeStatusTarget contains protected equality inputs and must never be logged
// or copied into an ordinary TopologyStatus.
type NodeStatusTarget struct {
	Slot                  string
	Role                  string
	SSHAlias              string
	HostKeyPinRef         string
	OperatorSignerAlias   string
	ExpectedNodePrincipal string
	ExpectedImage         string
	ExpectedIngress       string
	PersistentStore       bool
}

// NodeObservation is protected host-local truth returned by one adapter.
type NodeObservation struct {
	NodeName          string
	NodePrincipal     string
	RuntimeReady      bool
	RuntimeReason     string
	Joined            bool
	ReachabilityMode  string
	ReachabilityState string
	Reachable         bool
	StoreEnabled      bool
	StoreState        string
	ImageReference    string
}

// TopologyStatus is the deterministic redacted MR-02 aggregate.
type TopologyStatus struct {
	APIVersion string          `json:"api_version"`
	Outcome    TopologyOutcome `json:"outcome"`
	Nodes      []NodeStatus    `json:"nodes"`
}

// NodeStatus exposes stable slot-scoped truth only.
type NodeStatus struct {
	Slot         string               `json:"slot"`
	Role         string               `json:"role"`
	Observation  NodeObservationState `json:"observation"`
	Ready        bool                 `json:"ready"`
	Readiness    NodeTruth            `json:"readiness"`
	Joined       bool                 `json:"joined"`
	Reachability NodeTruth            `json:"reachability"`
	Store        NodeTruth            `json:"store"`
	Image        NodeImageState       `json:"image"`
	Reason       StatusReason         `json:"reason,omitempty"`
}

// StatusInspector validates one topology before querying exactly three Nodes.
type StatusInspector struct {
	Probe          NodeStatusProbe
	PerNodeTimeout time.Duration
}

// Inspect returns partial status as data; only invalid local input/configuration
// prevents an aggregate from being produced.
func (inspector StatusInspector) Inspect(ctx context.Context, raw []byte) (TopologyStatus, error) {
	if inspector.Probe == nil {
		return TopologyStatus{}, ValidationError("topology_status_probe_required")
	}
	timeout := inspector.PerNodeTimeout
	if timeout == 0 {
		timeout = defaultNodeStatusTimeout
	}
	if timeout < time.Millisecond || timeout > defaultNodeStatusTimeout {
		return TopologyStatus{}, ValidationError("topology_status_timeout_invalid")
	}
	manifest, err := decodeTopology(raw)
	if err != nil {
		return TopologyStatus{}, err
	}
	if err := validateTopology(manifest); err != nil {
		return TopologyStatus{}, err
	}
	targets := statusTargets(manifest)
	operationContext, cancelOperation := context.WithTimeout(ctx, maxTopologyStatusTimeout)
	defer cancelOperation()
	status := TopologyStatus{
		APIVersion: TopologyStatusVersion,
		Outcome:    TopologyOutcomeReady,
		Nodes:      make([]NodeStatus, 0, len(targets)),
	}
	for _, target := range targets {
		nodeContext, cancelNode := context.WithTimeout(operationContext, timeout)
		observation, observeErr := inspector.Probe.Observe(nodeContext, target)
		cancelNode()
		if observeErr != nil {
			status.Outcome = TopologyOutcomePartial
			status.Nodes = append(status.Nodes, unavailableNodeStatus(target, observeErr))
			continue
		}
		node := projectNodeStatus(target, observation)
		if !node.Ready && status.Outcome == TopologyOutcomeReady {
			status.Outcome = TopologyOutcomeDegraded
		}
		status.Nodes = append(status.Nodes, node)
	}
	return status, nil
}

func statusTargets(manifest topologyManifest) []NodeStatusTarget {
	targets := make([]NodeStatusTarget, 0, len(manifest.Nodes))
	for _, node := range manifest.Nodes {
		role := "member"
		if node.Slot == manifest.Authority.Slot {
			role = "authority"
		}
		targets = append(targets, NodeStatusTarget{
			Slot: node.Slot, Role: role,
			SSHAlias: node.Host.SSHAlias, HostKeyPinRef: node.Host.HostKeyPinRef,
			OperatorSignerAlias:   manifest.OperatorSignerAlias,
			ExpectedNodePrincipal: node.ExpectedNodePrincipal,
			ExpectedImage:         node.Image,
			ExpectedIngress:       node.Ingress.Kind, PersistentStore: node.Store.Persistent,
		})
	}
	sort.Slice(targets, func(left, right int) bool {
		return targets[left].Slot < targets[right].Slot
	})
	return targets
}

func unavailableNodeStatus(target NodeStatusTarget, err error) NodeStatus {
	return NodeStatus{
		Slot: target.Slot, Role: target.Role,
		Observation: NodeObservationUnavailable, Ready: false,
		Readiness: NodeTruthUnavailable, Reachability: NodeTruthUnavailable,
		Store: NodeTruthUnavailable, Image: NodeImageUnverified,
		Reason: probeReason(err),
	}
}

func probeReason(err error) StatusReason {
	if errors.Is(err, context.DeadlineExceeded) {
		return StatusReason(ProbeTunnelTimeout)
	}
	var probeErr ProbeError
	if errors.As(err, &probeErr) && validProbeErrorCode(ProbeErrorCode(probeErr)) {
		return StatusReason(probeErr)
	}
	return StatusReason(ProbeRemoteInvalidResponse)
}

func validProbeErrorCode(code ProbeErrorCode) bool {
	switch code {
	case ProbeHostKeyMismatch, ProbeTunnelTimeout, ProbeTunnelFailure,
		ProbeLocalSignerUnavailable, ProbeRemoteUnauthenticated,
		ProbeRemoteDenied, ProbeNodeUnavailable, ProbeRemoteInvalidResponse:
		return true
	default:
		return false
	}
}

func projectNodeStatus(target NodeStatusTarget, observation NodeObservation) NodeStatus {
	status := NodeStatus{
		Slot: target.Slot, Role: target.Role,
		Observation: NodeObservationComplete,
		Ready:       true, Readiness: NodeTruthReady, Joined: observation.Joined,
		Reachability: NodeTruthReady, Store: NodeTruthNotRequired,
		Image: NodeImageMatch,
	}
	if !validObservation(observation) {
		status.Ready = false
		status.Observation = NodeObservationUnavailable
		status.Readiness = NodeTruthUnavailable
		status.Reachability = NodeTruthUnavailable
		status.Store = NodeTruthUnavailable
		status.Image = NodeImageUnverified
		status.Reason = StatusReason(ProbeRemoteInvalidResponse)
		return status
	}
	principal, principalErr := identityprincipal.Parse(observation.NodePrincipal)
	if observation.NodeName != target.Slot || principalErr != nil ||
		principal.String() != target.ExpectedNodePrincipal {
		status.Ready = false
		status.Reason = StatusReasonNodeIdentityMismatch
	}
	switch observation.ImageReference {
	case "":
		status.Ready = false
		status.Image = NodeImageUnverified
		if status.Reason == "" {
			status.Reason = StatusReasonImageUnverified
		}
	case target.ExpectedImage:
		status.Image = NodeImageMatch
	default:
		status.Ready = false
		status.Image = NodeImageMismatch
		if status.Reason == "" {
			status.Reason = StatusReasonImageMismatch
		}
	}
	if !observation.RuntimeReady {
		status.Ready = false
		status.Readiness = NodeTruthDegraded
		if status.Reason == "" {
			status.Reason = StatusReasonCompositeReadinessDegraded
		}
	}
	if !observation.Joined {
		status.Ready = false
		status.Reachability = NodeTruthDegraded
		if status.Reason == "" {
			status.Reason = StatusReasonNetworkNotJoined
		}
	} else if !reachabilityReady(target, observation) {
		status.Ready = false
		status.Reachability = NodeTruthDegraded
		if status.Reason == "" {
			status.Reason = reachabilityReason(target)
		}
	}
	status.Store = storeTruth(target, observation)
	if status.Store == NodeTruthDegraded || status.Store == NodeTruthUnavailable {
		status.Ready = false
		if status.Reason == "" {
			if !observation.StoreEnabled {
				status.Reason = StatusReasonPersistentStoreUnavailable
			} else {
				status.Reason = StatusReasonPersistentStoreDegraded
			}
		}
	}
	return status
}

func validObservation(value NodeObservation) bool {
	for _, item := range []string{
		value.NodeName, value.NodePrincipal, value.RuntimeReason,
		value.ReachabilityMode, value.ReachabilityState,
		value.StoreState, value.ImageReference,
	} {
		if len(item) > maxObservationString || strings.ContainsRune(item, '\x00') {
			return false
		}
	}
	switch value.StoreState {
	case "ready", "degraded", "failed", "disabled":
	default:
		return false
	}
	return true
}

func reachabilityReady(target NodeStatusTarget, value NodeObservation) bool {
	switch target.ExpectedIngress {
	case "public":
		return value.ReachabilityMode == "public_direct" &&
			value.ReachabilityState == "public" && value.Reachable
	case "outbound_only":
		return value.ReachabilityMode == "outbound_only" &&
			value.ReachabilityState == "outbound_only" && !value.Reachable
	case "private_lan":
		return value.ReachabilityMode == "private_lan" &&
			value.ReachabilityState == "lan" && value.Reachable
	default:
		return false
	}
}

func reachabilityReason(target NodeStatusTarget) StatusReason {
	switch target.ExpectedIngress {
	case "public":
		return StatusReasonPublicReachabilityUnverified
	case "private_lan":
		return StatusReasonPrivateReachabilityUnverified
	default:
		return StatusReasonOutboundReachabilityMismatch
	}
}

func storeTruth(target NodeStatusTarget, value NodeObservation) NodeTruth {
	if !target.PersistentStore {
		if value.StoreEnabled {
			return NodeTruthDegraded
		}
		return NodeTruthNotRequired
	}
	if !value.StoreEnabled {
		return NodeTruthUnavailable
	}
	if value.StoreState != "ready" {
		return NodeTruthDegraded
	}
	return NodeTruthReady
}
