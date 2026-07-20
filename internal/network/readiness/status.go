package readiness

import (
	networkmessaging "ardents/internal/network/messaging"
	networkpeer "ardents/internal/network/peer"

	wakuNode "github.com/waku-org/go-waku/waku/v2/node"
)

type HealthSignals struct {
	NodeProfile          NodeProfile
	ServiceState         string
	ServiceReason        string
	BootstrapSourceCount int
	BootstrapStatus      BootstrapStatus
	EndpointCount        int
	UsableEndpointCount  int
	PeerCount            int
	RelayPeerCount       int
	FilterPeerCount      int
	LightpushPeerCount   int
	StorePeerCount       int
	Reachability         ReachabilitySnapshot
}

type runtimeAssessment struct {
	state               string
	reason              string
	health              HealthState
	switchReason        SwitchReason
	switchAutomatic     bool
	reducedCapabilities []string
	recoveryState       RecoveryState
}

func (r runtimeAssessment) Health() HealthState {
	return r.health
}

func (r runtimeAssessment) SwitchReason() SwitchReason {
	return r.switchReason
}

func (r runtimeAssessment) SwitchAutomatic() bool {
	return r.switchAutomatic
}

func (r runtimeAssessment) RecoveryState() RecoveryState {
	return r.recoveryState
}

type ServiceState struct {
	NodeProfile        NodeProfile
	State              string
	Reason             string
	BootstrapNodes     []string
	Endpoints          []string
	ObservedUsable     int
	NodePeerCount      int
	NodeRelayCount     int
	FilterPeerCount    int
	LightpushPeerCount int
	StorePeerCount     int
	Bootstrap          BootstrapStatus
	ActiveProfile      Profile
	ActiveMode         Mode
	SwitchReason       SwitchReason
	SwitchAutomatic    bool
	RecoveryState      RecoveryState
	Reachability       ReachabilitySnapshot
}

type ServicePartSnapshot struct {
	State  string
	Reason string
}

func DefaultPubsubTopic() string {
	return networkmessaging.DefaultPubsubTopic
}

func CurrentBootstrapStatus(node *wakuNode.WakuNode, bootstrapNodes []string, current BootstrapStatus) BootstrapStatus {
	if node == nil {
		return current
	}
	attempts := countBootstrapPeers(bootstrapNodes)
	if attempts == 0 {
		return BootstrapStatus{State: "idle", Reason: "no network bootstrap sources"}
	}
	if len(node.Relay().PubSub().ListPeers(networkmessaging.DefaultPubsubTopic)) > 0 {
		return BootstrapStatus{Joined: true, State: "ready"}
	}
	reason := current.Reason
	if reason == "" || current.State == "ready" {
		reason = "bootstrap relay path is not operational"
	}
	return BootstrapStatus{State: "degraded", Reason: reason}
}

func ProfileSnapshot(state ServiceState) Snapshot {
	profile := NormalizeProfile(state.ActiveProfile)
	snapshot := Snapshot{NodeProfile: state.NodeProfile, Profile: profile, Mode: state.ActiveMode}
	definition, err := ResolveProfile(profile)
	if err != nil {
		snapshot.Health = HealthStateFailed
		snapshot.SwitchReason = SwitchReasonStartupFailed
		snapshot.RecoveryState = RecoveryStateBlocked
		snapshot.ReducedCapabilities = []string{"relay", "store", "filter", "lightpush"}
		return snapshot
	}
	assessment := RuntimeAssessment(state)
	snapshot.ActiveFamilies = cloneFamilies(definition.ActiveFamilies)
	snapshot.SuppressedFamilies = cloneFamilies(definition.SuppressedFamilies)
	snapshot.Health = assessment.health
	snapshot.SwitchReason = assessment.switchReason
	snapshot.SwitchAutomatic = assessment.switchAutomatic
	snapshot.RecoveryState = assessment.recoveryState
	snapshot.ReducedCapabilities = cloneStrings(assessment.reducedCapabilities)
	snapshot.ActiveCapabilities = activeMessagingCapabilities(state)
	return snapshot
}

func BuildServicePartSnapshot(state ServiceState) ServicePartSnapshot {
	assessment := RuntimeAssessment(state)
	return ServicePartSnapshot{State: assessment.state, Reason: assessment.reason}
}

func HealthSnapshot(state ServiceState) HealthSignals {
	return HealthSignals{
		NodeProfile:          state.NodeProfile,
		ServiceState:         state.State,
		ServiceReason:        state.Reason,
		BootstrapSourceCount: countBootstrapPeers(state.BootstrapNodes),
		BootstrapStatus:      state.Bootstrap,
		EndpointCount:        len(state.Endpoints),
		UsableEndpointCount:  state.ObservedUsable,
		PeerCount:            state.NodePeerCount,
		RelayPeerCount:       state.NodeRelayCount,
		FilterPeerCount:      state.FilterPeerCount,
		LightpushPeerCount:   state.LightpushPeerCount,
		StorePeerCount:       state.StorePeerCount,
		Reachability:         state.Reachability,
	}
}

func RuntimeAssessment(state ServiceState) runtimeAssessment {
	raw := BaseRuntimeAssessment(state)
	applied := raw
	if state.SwitchReason != "" {
		applied.switchReason = state.SwitchReason
		applied.switchAutomatic = state.SwitchAutomatic
	}
	if state.RecoveryState != "" {
		applied.recoveryState = state.RecoveryState
	}
	if state.ActiveMode != ModeRestrictedDefense {
		return applied
	}
	if raw.health == HealthStateFailed || raw.health == HealthStateStopped || raw.health == HealthStateStarting {
		return applied
	}
	applied.state = "degraded"
	applied.health = HealthStateDegraded
	if applied.reason == "" {
		applied.reason = "restricted defense mode is active"
	}
	applied.reducedCapabilities = appendUniqueCapabilities(
		applied.reducedCapabilities,
		"store",
		"filter_service",
		"lightpush_service",
		"transport_surface_expansion",
		"profile_recovery_pending",
	)
	if applied.recoveryState == "" {
		applied.recoveryState = RecoveryStateRecoveryPending
	}
	return applied
}

func BaseRuntimeAssessment(state ServiceState) runtimeAssessment {
	return classifyHealth(HealthSnapshot(state))
}

func classifyHealth(signals HealthSignals) runtimeAssessment {
	switch signals.ServiceState {
	case "failed":
		return failedAssessment(signals.ServiceReason)
	case "stopped":
		return runtimeAssessment{state: "stopped", health: HealthStateStopped, switchReason: SwitchReasonStopped}
	case "degraded":
		return degradedAssessment(signals)
	case "ready":
		return readyAssessment(signals)
	default:
		return runtimeAssessment{
			state:        signals.ServiceState,
			reason:       signals.ServiceReason,
			health:       HealthStateStarting,
			switchReason: SwitchReasonNotStarted,
		}
	}
}

func failedAssessment(reason string) runtimeAssessment {
	return runtimeAssessment{
		state:               "failed",
		reason:              reason,
		health:              HealthStateFailed,
		switchReason:        SwitchReasonStartupFailed,
		reducedCapabilities: []string{"relay", "store", "filter", "lightpush"},
		recoveryState:       RecoveryStateBlocked,
	}
}

func degradedAssessment(signals HealthSignals) runtimeAssessment {
	reason := signals.BootstrapStatus.Reason
	if reason == "" {
		reason = signals.ServiceReason
	}
	return runtimeAssessment{
		state:               "degraded",
		reason:              reason,
		health:              HealthStateDegraded,
		switchReason:        SwitchReasonBootstrapDegraded,
		switchAutomatic:     true,
		reducedCapabilities: []string{"peer_connectivity", "bootstrap_sync"},
		recoveryState:       RecoveryStateRecoveryPending,
	}
}

func readyAssessment(signals HealthSignals) runtimeAssessment {
	if signals.EndpointCount == 0 || signals.UsableEndpointCount == 0 {
		if signals.Reachability.Mode == ReachabilityOutboundOnly {
			return readyWithBootstrap(signals)
		}
		if signals.Reachability.Mode == ReachabilityPublicDirect {
			reason := signals.Reachability.Reason
			if reason == "" {
				reason = "public ingress has not been verified"
			}
			return runtimeAssessment{
				state:               "degraded",
				reason:              reason,
				health:              HealthStateDegraded,
				switchReason:        SwitchReasonBootstrapDegraded,
				switchAutomatic:     true,
				reducedCapabilities: []string{"inbound_reachability", "endpoint_publication"},
				recoveryState:       RecoveryStateRecoveryPending,
			}
		}
		return runtimeAssessment{
			state:               "degraded",
			reason:              "transport endpoints are not operational",
			health:              HealthStateDegraded,
			switchReason:        SwitchReasonBootstrapDegraded,
			switchAutomatic:     true,
			reducedCapabilities: []string{"peer_connectivity"},
			recoveryState:       RecoveryStateRecoveryPending,
		}
	}
	return readyWithBootstrap(signals)
}

func readyWithBootstrap(signals HealthSignals) runtimeAssessment {
	if signals.NodeProfile == NodeProfileConstrainedClient && signals.BootstrapSourceCount == 0 {
		return runtimeAssessment{
			state: "degraded", reason: "constrained light client has no Filter, Lightpush, or Store providers",
			health: HealthStateDegraded, switchReason: SwitchReasonBootstrapDegraded, switchAutomatic: true,
			reducedCapabilities: []string{"filter", "lightpush", "store_recovery"}, recoveryState: RecoveryStateRecoveryPending,
		}
	}
	if signals.BootstrapSourceCount == 0 || signals.BootstrapStatus.State == "ready" {
		return runtimeAssessment{
			state:         "ready",
			health:        HealthStateReady,
			switchReason:  SwitchReasonStartupDefault,
			recoveryState: RecoveryStateSteady,
		}
	}
	reason := signals.BootstrapStatus.Reason
	if reason == "" {
		reason = "bootstrap relay path is not operational"
	}
	return runtimeAssessment{
		state:               "degraded",
		reason:              reason,
		health:              HealthStateDegraded,
		switchReason:        SwitchReasonBootstrapDegraded,
		switchAutomatic:     true,
		reducedCapabilities: []string{"peer_connectivity", "bootstrap_sync"},
		recoveryState:       RecoveryStateRecoveryPending,
	}
}

func countBootstrapPeers(peers []string) int {
	count := 0
	for _, peer := range peers {
		if _, ok := networkpeer.Normalize(peer); ok {
			count++
		}
	}
	return count
}
