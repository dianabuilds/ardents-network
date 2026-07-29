package deployment

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
)

const (
	// AuthorityRecoveryVersion identifies the bounded redacted MR-03 result.
	AuthorityRecoveryVersion = "ardents.topology.authority-recovery/v1"

	AuthorityRecoveryOutcomeAlreadyReady     AuthorityRecoveryOutcome = "already_ready"
	AuthorityRecoveryOutcomeVerified         AuthorityRecoveryOutcome = "verified"
	AuthorityRecoveryOutcomeRecoveryRequired AuthorityRecoveryOutcome = "recovery_required"

	AuthorityRecoveryReasonClockUnavailable       AuthorityRecoveryReason = "clock_unavailable"
	AuthorityRecoveryReasonClockSkew              AuthorityRecoveryReason = "clock_skew_exceeded"
	AuthorityRecoveryReasonAuthorityUnavailable   AuthorityRecoveryReason = "authority_unavailable"
	AuthorityRecoveryReasonAuthorityDenied        AuthorityRecoveryReason = "authority_denied"
	AuthorityRecoveryReasonAuthorityInvalid       AuthorityRecoveryReason = "authority_invalid_response"
	AuthorityRecoveryReasonAuthorityState         AuthorityRecoveryReason = "authority_state_mismatch"
	AuthorityRecoveryReasonCheckpointMismatch     AuthorityRecoveryReason = "checkpoint_verification_mismatch"
	AuthorityRecoveryReasonRepositoryUnavailable  AuthorityRecoveryReason = "checkpoint_repository_unavailable"
	AuthorityRecoveryReasonCheckpointMissing      AuthorityRecoveryReason = "checkpoint_head_missing"
	AuthorityRecoveryReasonCheckpointHeadMismatch AuthorityRecoveryReason = "checkpoint_head_mismatch"
	AuthorityRecoveryReasonPersistedStateInvalid  AuthorityRecoveryReason = "authority_state_invalid"
	AuthorityRecoveryReasonSignerMismatch         AuthorityRecoveryReason = "authority_signer_mismatch"

	AuthorityChangeCompatible AuthorityChangeKind = "compatible_release"
	AuthorityChangeMigration  AuthorityChangeKind = "authority_migration"

	maxAuthorityObservationString = 512
)

// AuthorityRecoveryOutcome is the closed recovery result vocabulary.
type AuthorityRecoveryOutcome string

// AuthorityRecoveryReason is one redacted fail-closed recovery reason.
type AuthorityRecoveryReason string

// AuthorityRecoveryProbeError preserves one stable remote Authority failure
// class without carrying an underlying message or protected detail.
type AuthorityRecoveryProbeError AuthorityRecoveryReason

func (err AuthorityRecoveryProbeError) Error() string { return string(err) }

// AuthorityChangeKind selects one accepted serial rollout order.
type AuthorityChangeKind string

// AuthorityRecoveryStatus is the ordinary redacted topology recovery result.
type AuthorityRecoveryStatus struct {
	APIVersion string                   `json:"api_version"`
	Slot       string                   `json:"slot"`
	Outcome    AuthorityRecoveryOutcome `json:"outcome"`
	Readiness  string                   `json:"readiness"`
	Phase      string                   `json:"phase"`
	Reason     AuthorityRecoveryReason  `json:"reason,omitempty"`
}

// AuthorityRecoveryTarget contains protected manifest bindings. It must never
// be rendered or logged as an ordinary result.
type AuthorityRecoveryTarget struct {
	Slot                         string
	Role                         string
	SSHAlias                     string
	HostKeyPinRef                string
	OperatorSignerAlias          string
	ExpectedNodePrincipal        string
	AuthorityStateRef            string
	AuthorityBackupRef           string
	CheckpointRepositoryRef      string
	MaxClockSkewSeconds          int
	AuthoritySafetyMarginSeconds int
}

// ClockObservation brackets one authenticated remote UTC observation with
// local request times so the coordinator can prove a conservative skew bound.
type ClockObservation struct {
	RequestStarted   time.Time
	ServerObservedAt time.Time
	ResponseReceived time.Time
}

// AuthorityObservation is protected DR-03 truth. Identifiers are used only
// for exact equality and are excluded from AuthorityRecoveryStatus.
type AuthorityObservation struct {
	RealmID           string
	AuthoritySequence uint64
	CheckpointDigest  string
	Phase             string
	Readiness         string
	Reason            string
}

// AuthorityRecoveryAcknowledgement carries the exact observed recovery tuple.
type AuthorityRecoveryAcknowledgement struct {
	RealmID           string
	AuthoritySequence uint64
	CheckpointDigest  string
}

// AuthorityRecoveryNode is one isolated Node/pin/session boundary.
type AuthorityRecoveryNode interface {
	ObserveClock(context.Context) (ClockObservation, error)
	InspectAuthority(context.Context) (AuthorityObservation, error)
	VerifyRestoredAuthority(context.Context, AuthorityRecoveryAcknowledgement) (AuthorityObservation, error)
	Close(context.Context) error
}

// AuthorityRecoveryProbe opens one isolated client for a manifest-bound Node.
type AuthorityRecoveryProbe interface {
	Open(context.Context, AuthorityRecoveryTarget) (AuthorityRecoveryNode, error)
}

// AuthorityRecoveryInspector coordinates only protected observations and the
// exact DR-03 verification acknowledgement. It never owns Authority storage.
type AuthorityRecoveryInspector struct {
	Probe          AuthorityRecoveryProbe
	PerNodeTimeout time.Duration
}

// Recover validates the manifest before opening exactly three Node clients.
func (inspector AuthorityRecoveryInspector) Recover(
	ctx context.Context,
	raw []byte,
) (AuthorityRecoveryStatus, error) {
	if inspector.Probe == nil {
		return AuthorityRecoveryStatus{}, ValidationError("topology_authority_recovery_probe_required")
	}
	timeout := inspector.PerNodeTimeout
	if timeout == 0 {
		timeout = defaultNodeStatusTimeout
	}
	if timeout < time.Millisecond || timeout > defaultNodeStatusTimeout {
		return AuthorityRecoveryStatus{}, ValidationError("topology_authority_recovery_timeout_invalid")
	}
	manifest, err := decodeTopology(raw)
	if err != nil {
		return AuthorityRecoveryStatus{}, err
	}
	if err := validateTopology(manifest); err != nil {
		return AuthorityRecoveryStatus{}, err
	}
	targets := authorityRecoveryTargets(manifest)
	operationContext, cancelOperation := context.WithTimeout(ctx, maxTopologyStatusTimeout)
	defer cancelOperation()
	observations := make([]ClockObservation, 0, len(targets))
	var authorityNode AuthorityRecoveryNode
	var authorityContext context.Context
	var cancelAuthority context.CancelFunc
	for _, target := range targets {
		nodeContext, cancelNode := context.WithTimeout(operationContext, timeout)
		node, openErr := inspector.Probe.Open(nodeContext, target)
		if openErr != nil || node == nil {
			cancelNode()
			return authorityRecoveryFailure(manifest.Authority.Slot, recoveryReason(openErr)), nil
		}
		clock, clockErr := node.ObserveClock(nodeContext)
		if clockErr != nil {
			_ = node.Close(nodeContext)
			cancelNode()
			return authorityRecoveryFailure(manifest.Authority.Slot, AuthorityRecoveryReasonClockUnavailable), nil
		}
		observations = append(observations, clock)
		if target.Role == "authority" {
			authorityNode, authorityContext, cancelAuthority = node, nodeContext, cancelNode
			continue
		}
		closeErr := node.Close(nodeContext)
		cancelNode()
		if closeErr != nil {
			return authorityRecoveryFailure(manifest.Authority.Slot, recoveryReason(closeErr)), nil
		}
	}
	if authorityNode == nil {
		return AuthorityRecoveryStatus{}, ValidationError("topology_invalid_authority_slot")
	}
	finished := false
	defer func() {
		if !finished {
			_ = authorityNode.Close(authorityContext)
			cancelAuthority()
		}
	}()
	finish := func(status AuthorityRecoveryStatus) AuthorityRecoveryStatus {
		closeErr := authorityNode.Close(authorityContext)
		cancelAuthority()
		finished = true
		if closeErr != nil && status.Outcome != AuthorityRecoveryOutcomeRecoveryRequired {
			return authorityRecoveryFailure(manifest.Authority.Slot, recoveryReason(closeErr))
		}
		return status
	}
	if !clockSkewWithin(observations, time.Duration(manifest.Clock.MaxSkewSeconds)*time.Second, timeout) {
		return finish(authorityRecoveryFailure(manifest.Authority.Slot, AuthorityRecoveryReasonClockSkew)), nil
	}
	observed, inspectErr := authorityNode.InspectAuthority(authorityContext)
	if inspectErr != nil {
		return finish(authorityRecoveryFailure(manifest.Authority.Slot, recoveryReason(inspectErr))), nil
	}
	if !validAuthorityObservation(observed) {
		return finish(authorityRecoveryFailure(manifest.Authority.Slot, AuthorityRecoveryReasonAuthorityInvalid)), nil
	}
	if observed.Phase == "ready" && observed.Readiness == "ready" && observed.Reason == "" {
		return finish(AuthorityRecoveryStatus{
			APIVersion: AuthorityRecoveryVersion, Slot: manifest.Authority.Slot,
			Outcome:   AuthorityRecoveryOutcomeAlreadyReady,
			Readiness: observed.Readiness, Phase: observed.Phase,
		}), nil
	}
	if observed.Phase != "recovery_only" || observed.Readiness != "degraded" ||
		observed.Reason != "authority_restore_verification_required" {
		return finish(authorityRecoveryFailure(
			manifest.Authority.Slot, authorityStateReason(observed.Reason),
		)), nil
	}
	acknowledgement := AuthorityRecoveryAcknowledgement{
		RealmID: observed.RealmID, AuthoritySequence: observed.AuthoritySequence,
		CheckpointDigest: observed.CheckpointDigest,
	}
	verified, verifyErr := authorityNode.VerifyRestoredAuthority(
		authorityContext, acknowledgement,
	)
	if verifyErr != nil {
		return finish(authorityRecoveryFailure(manifest.Authority.Slot, recoveryReason(verifyErr))), nil
	}
	if !sameVerifiedAuthority(observed, verified) {
		return finish(authorityRecoveryFailure(manifest.Authority.Slot, AuthorityRecoveryReasonCheckpointMismatch)), nil
	}
	return finish(AuthorityRecoveryStatus{
		APIVersion: AuthorityRecoveryVersion, Slot: manifest.Authority.Slot,
		Outcome:   AuthorityRecoveryOutcomeVerified,
		Readiness: verified.Readiness, Phase: verified.Phase,
	}), nil
}

func authorityRecoveryTargets(manifest topologyManifest) []AuthorityRecoveryTarget {
	targets := make([]AuthorityRecoveryTarget, 0, len(manifest.Nodes))
	for _, node := range manifest.Nodes {
		role := "member"
		target := AuthorityRecoveryTarget{
			Slot: node.Slot, Role: role, SSHAlias: node.Host.SSHAlias,
			HostKeyPinRef:                node.Host.HostKeyPinRef,
			OperatorSignerAlias:          manifest.OperatorSignerAlias,
			ExpectedNodePrincipal:        node.ExpectedNodePrincipal,
			MaxClockSkewSeconds:          manifest.Clock.MaxSkewSeconds,
			AuthoritySafetyMarginSeconds: manifest.Clock.AuthoritySafetyMarginSeconds,
		}
		if node.Slot == manifest.Authority.Slot {
			target.Role = "authority"
			target.AuthorityStateRef = manifest.Authority.StateRef
			target.AuthorityBackupRef = manifest.Authority.BackupRef
			target.CheckpointRepositoryRef = manifest.CheckpointRepository.Reference
		}
		targets = append(targets, target)
	}
	sort.Slice(targets, func(left, right int) bool {
		if targets[left].Role != targets[right].Role {
			return targets[left].Role == "member"
		}
		return targets[left].Slot < targets[right].Slot
	})
	return targets
}

func clockSkewWithin(observations []ClockObservation, maximum, timeout time.Duration) bool {
	if len(observations) != exactTopologyNodeCount || maximum <= 0 {
		return false
	}
	var minimumLower, maximumUpper time.Duration
	for index, observation := range observations {
		if observation.RequestStarted.IsZero() || observation.ServerObservedAt.IsZero() ||
			observation.ResponseReceived.IsZero() ||
			observation.ResponseReceived.Before(observation.RequestStarted) ||
			observation.ResponseReceived.Sub(observation.RequestStarted) > timeout {
			return false
		}
		lower := observation.ServerObservedAt.Sub(observation.ResponseReceived)
		upper := observation.ServerObservedAt.Sub(observation.RequestStarted)
		if index == 0 || lower < minimumLower {
			minimumLower = lower
		}
		if index == 0 || upper > maximumUpper {
			maximumUpper = upper
		}
	}
	return maximumUpper-minimumLower <= maximum
}

func validAuthorityObservation(observation AuthorityObservation) bool {
	if observation.AuthoritySequence == 0 || observation.RealmID == "" ||
		observation.CheckpointDigest == "" {
		return false
	}
	for _, value := range []string{
		observation.RealmID, observation.CheckpointDigest, observation.Phase,
		observation.Readiness, observation.Reason,
	} {
		if len(value) > maxAuthorityObservationString || strings.ContainsRune(value, '\x00') {
			return false
		}
	}
	return true
}

func sameVerifiedAuthority(before, after AuthorityObservation) bool {
	return validAuthorityObservation(after) &&
		after.RealmID == before.RealmID &&
		after.AuthoritySequence == before.AuthoritySequence &&
		after.CheckpointDigest == before.CheckpointDigest &&
		after.Phase == "ready" && after.Readiness == "ready" && after.Reason == ""
}

func recoveryReason(err error) AuthorityRecoveryReason {
	if err == nil {
		return AuthorityRecoveryReasonAuthorityUnavailable
	}
	var authorityErr AuthorityRecoveryProbeError
	if errors.As(err, &authorityErr) {
		return AuthorityRecoveryReason(authorityErr)
	}
	var probeErr ProbeError
	if !asProbeError(err, &probeErr) {
		return AuthorityRecoveryReasonAuthorityUnavailable
	}
	switch ProbeErrorCode(probeErr) {
	case ProbeRemoteDenied:
		return AuthorityRecoveryReasonAuthorityDenied
	case ProbeRemoteInvalidResponse:
		return AuthorityRecoveryReasonAuthorityInvalid
	default:
		return AuthorityRecoveryReason(string(probeErr))
	}
}

func authorityStateReason(reason string) AuthorityRecoveryReason {
	switch reason {
	case string(AuthorityRecoveryReasonRepositoryUnavailable):
		return AuthorityRecoveryReasonRepositoryUnavailable
	case string(AuthorityRecoveryReasonCheckpointMissing):
		return AuthorityRecoveryReasonCheckpointMissing
	case string(AuthorityRecoveryReasonCheckpointHeadMismatch):
		return AuthorityRecoveryReasonCheckpointHeadMismatch
	case string(AuthorityRecoveryReasonPersistedStateInvalid):
		return AuthorityRecoveryReasonPersistedStateInvalid
	case string(AuthorityRecoveryReasonSignerMismatch):
		return AuthorityRecoveryReasonSignerMismatch
	default:
		return AuthorityRecoveryReasonAuthorityState
	}
}

func asProbeError(err error, target *ProbeError) bool {
	return errors.As(err, target)
}

func authorityRecoveryFailure(slot string, reason AuthorityRecoveryReason) AuthorityRecoveryStatus {
	return AuthorityRecoveryStatus{
		APIVersion: AuthorityRecoveryVersion, Slot: slot,
		Outcome:   AuthorityRecoveryOutcomeRecoveryRequired,
		Readiness: "recovery_required", Phase: "recovery_required", Reason: reason,
	}
}

// AuthorityRolloutOrder returns the deterministic serial order selected by
// ADR-0013 without mutating a host or Authority state.
func AuthorityRolloutOrder(raw []byte, kind AuthorityChangeKind) ([]string, error) {
	manifest, err := decodeTopology(raw)
	if err != nil {
		return nil, err
	}
	if err := validateTopology(manifest); err != nil {
		return nil, err
	}
	if kind != AuthorityChangeCompatible && kind != AuthorityChangeMigration {
		return nil, ValidationError("topology_invalid_authority_change_kind")
	}
	members := make([]string, 0, exactTopologyNodeCount-1)
	for _, node := range manifest.Nodes {
		if node.Slot != manifest.Authority.Slot {
			members = append(members, node.Slot)
		}
	}
	sort.Strings(members)
	if kind == AuthorityChangeMigration {
		return append([]string{manifest.Authority.Slot}, members...), nil
	}
	return append(members, manifest.Authority.Slot), nil
}
