package topology

import (
	"context"
	"errors"
	"time"

	"ardents/internal/authority"
	"ardents/internal/cli/client"
	configurationcmd "ardents/internal/cli/configuration"
	"ardents/internal/deployment"
	protocol "ardents/internal/localapi/protocol"

	"connectrpc.com/connect"
)

// RecoveryProbe binds one reviewed topology target to one protected client.
type RecoveryProbe struct {
	Base    configurationcmd.Config
	Factory clientFactory
}

func (probe RecoveryProbe) Open(
	_ context.Context,
	target deployment.AuthorityRecoveryTarget,
) (deployment.AuthorityRecoveryNode, error) {
	resolved, err := probe.Base.ResolveTopologyContext(target.SSHAlias)
	if err != nil {
		return nil, deployment.ProbeError(deployment.ProbeTunnelFailure)
	}
	if resolved.ContextName != target.SSHAlias ||
		resolved.HostKeyPinRef != target.HostKeyPinRef {
		return nil, deployment.ProbeError(deployment.ProbeHostKeyMismatch)
	}
	if resolved.SignerAlias != target.OperatorSignerAlias {
		return nil, deployment.ProbeError(deployment.ProbeLocalSignerUnavailable)
	}
	if resolved.ExpectedNode != target.Slot ||
		resolved.ExpectedPrincipal != target.ExpectedNodePrincipal {
		return nil, deployment.ProbeError(deployment.ProbeRemoteInvalidResponse)
	}
	if target.Role == "authority" {
		if !authority.ValidRealmID(resolved.ExpectedRealm) ||
			resolved.AuthorityStateRef != target.AuthorityStateRef ||
			resolved.AuthorityBackupRef != target.AuthorityBackupRef ||
			resolved.CheckpointRepositoryRef != target.CheckpointRepositoryRef {
			return nil, deployment.ProbeError(deployment.ProbeRemoteInvalidResponse)
		}
	}
	factory := probe.Factory
	if factory == nil {
		factory = operatorClientFactory{}
	}
	opened, err := factory.Open(resolved)
	if err != nil {
		return nil, classifyProbeError(err)
	}
	if opened.calls == nil || opened.close == nil {
		return nil, deployment.ProbeError(deployment.ProbeRemoteInvalidResponse)
	}
	return &recoveryNode{
		calls: opened.calls, close: opened.close, target: target,
		expectedRealm: resolved.ExpectedRealm,
	}, nil
}

type recoveryNode struct {
	calls         protectedCalls
	close         func(context.Context) error
	target        deployment.AuthorityRecoveryTarget
	expectedRealm string
}

func (node *recoveryNode) ObserveClock(
	ctx context.Context,
) (deployment.ClockObservation, error) {
	started := time.Now().UTC()
	response, err := node.calls.GetNodeRuntime(
		ctx, client.Request(&protocol.GetNodeRuntimeRequest{}),
	)
	received := time.Now().UTC()
	if err != nil {
		return deployment.ClockObservation{}, classifyProbeError(err)
	}
	if response == nil || response.Msg == nil ||
		response.Msg.GetRuntime() == nil ||
		response.Msg.GetRuntime().GetNode() == nil ||
		response.Msg.GetRuntime().GetIdentity() == nil ||
		response.Msg.GetObservedAt() == nil ||
		response.Msg.GetObservedAt().CheckValid() != nil {
		return deployment.ClockObservation{}, deployment.ProbeError(deployment.ProbeRemoteInvalidResponse)
	}
	runtime := response.Msg.GetRuntime()
	if runtime.GetNode().GetName() != node.target.Slot ||
		runtime.GetIdentity().GetPrincipal() != node.target.ExpectedNodePrincipal {
		return deployment.ClockObservation{}, deployment.ProbeError(deployment.ProbeRemoteInvalidResponse)
	}
	return deployment.ClockObservation{
		RequestStarted: started, ServerObservedAt: response.Msg.GetObservedAt().AsTime().UTC(),
		ResponseReceived: received,
	}, nil
}

func (node *recoveryNode) InspectAuthority(
	ctx context.Context,
) (deployment.AuthorityObservation, error) {
	if node.target.Role != "authority" || !authority.ValidRealmID(node.expectedRealm) {
		return deployment.AuthorityObservation{}, deployment.ProbeError(deployment.ProbeRemoteInvalidResponse)
	}
	response, err := node.calls.InspectRealmAuthority(ctx, client.Request(
		&protocol.InspectRealmAuthorityRequest{
			Version: authority.ContractVersion, RealmId: node.expectedRealm,
		},
	))
	if err != nil {
		return deployment.AuthorityObservation{}, classifyAuthorityRecoveryError(err)
	}
	if response == nil || response.Msg == nil || response.Msg.GetAuthority() == nil {
		return deployment.AuthorityObservation{}, deployment.ProbeError(deployment.ProbeRemoteInvalidResponse)
	}
	observation := authorityObservation(response.Msg.GetAuthority())
	if !validAuthorityResponse(node.expectedRealm, observation) {
		return deployment.AuthorityObservation{}, deployment.ProbeError(deployment.ProbeRemoteInvalidResponse)
	}
	return observation, nil
}

func (node *recoveryNode) VerifyRestoredAuthority(
	ctx context.Context,
	ack deployment.AuthorityRecoveryAcknowledgement,
) (deployment.AuthorityObservation, error) {
	if node.target.Role != "authority" || ack.RealmID != node.expectedRealm {
		return deployment.AuthorityObservation{}, deployment.ProbeError(deployment.ProbeRemoteInvalidResponse)
	}
	response, err := node.calls.VerifyRestoredAuthority(ctx, client.Request(
		&protocol.VerifyRestoredAuthorityRequest{
			Version: authority.ContractVersion, RealmId: ack.RealmID,
			AuthoritySequence: ack.AuthoritySequence, CheckpointDigest: ack.CheckpointDigest,
		},
	))
	if err != nil {
		return deployment.AuthorityObservation{}, classifyAuthorityRecoveryError(err)
	}
	if response == nil || response.Msg == nil || response.Msg.GetAuthority() == nil {
		return deployment.AuthorityObservation{}, deployment.ProbeError(deployment.ProbeRemoteInvalidResponse)
	}
	observation := authorityObservation(response.Msg.GetAuthority())
	if !validAuthorityResponse(node.expectedRealm, observation) {
		return deployment.AuthorityObservation{}, deployment.ProbeError(deployment.ProbeRemoteInvalidResponse)
	}
	return observation, nil
}

func (node *recoveryNode) Close(ctx context.Context) error {
	return classifyProbeError(node.close(ctx))
}

func authorityObservation(status *protocol.AuthorityStatusSnapshot) deployment.AuthorityObservation {
	return deployment.AuthorityObservation{
		RealmID: status.GetRealmId(), AuthoritySequence: status.GetAuthoritySequence(),
		CheckpointDigest: status.GetCheckpointDigest(), Phase: status.GetPhase(),
		Readiness: status.GetReadiness(), Reason: status.GetReason(),
	}
}

func validAuthorityResponse(
	expectedRealm string,
	observation deployment.AuthorityObservation,
) bool {
	return observation.RealmID == expectedRealm &&
		observation.AuthoritySequence > 0 &&
		observation.AuthoritySequence <= authority.MaxCheckpointRecords &&
		authority.ValidCheckpointDigest(observation.CheckpointDigest)
}

func classifyAuthorityRecoveryError(err error) error {
	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		for _, detail := range connectErr.Details() {
			message, detailErr := detail.Value()
			apiError, ok := message.(*protocol.Error)
			if detailErr != nil || !ok {
				continue
			}
			switch apiError.GetReason() {
			case "authority_forbidden":
				return deployment.AuthorityRecoveryProbeError(
					deployment.AuthorityRecoveryReasonAuthorityDenied,
				)
			case "authority_recovery_required", "authority_conflict":
				return deployment.AuthorityRecoveryProbeError(
					deployment.AuthorityRecoveryReasonCheckpointMismatch,
				)
			case "authority_unavailable":
				return deployment.AuthorityRecoveryProbeError(
					deployment.AuthorityRecoveryReasonAuthorityUnavailable,
				)
			case "authority_resource_exhausted":
				return deployment.AuthorityRecoveryProbeError(
					deployment.AuthorityRecoveryReasonAuthorityState,
				)
			}
		}
	}
	return classifyProbeError(err)
}
