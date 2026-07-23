package waku

import (
	"ardents/internal/network"
	networkpeer "ardents/internal/network/peer"

	wakuNode "github.com/waku-org/go-waku/waku/v2/node"
)

type runtimeAssessment struct {
	state           string
	reason          string
	health          network.HealthState
	switchReason    network.SwitchReason
	switchAutomatic bool
	reducedFeatures []network.TransportFeature
	recoveryState   network.RecoveryState
}

func (r runtimeAssessment) Health() network.HealthState {
	return r.health
}

func (r runtimeAssessment) SwitchReason() network.SwitchReason {
	return r.switchReason
}

func (r runtimeAssessment) SwitchAutomatic() bool {
	return r.switchAutomatic
}

func (r runtimeAssessment) RecoveryState() network.RecoveryState {
	return r.recoveryState
}

type ServiceState struct {
	NodeProfile        network.NodeProfile
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
	Bootstrap          network.BootstrapStatus
	ActiveProfile      network.Profile
	ActiveMode         network.Mode
	SwitchReason       network.SwitchReason
	SwitchAutomatic    bool
	RecoveryState      network.RecoveryState
	Reachability       network.ReachabilitySnapshot
}

type ServicePartSnapshot struct {
	State  string
	Reason string
}

func CurrentBootstrapStatus(node *wakuNode.WakuNode, bootstrapNodes []string, current network.BootstrapStatus) network.BootstrapStatus {
	if node == nil {
		return current
	}
	attempts := countBootstrapPeers(bootstrapNodes)
	if attempts == 0 {
		return network.BootstrapStatus{State: "idle", Reason: "no network bootstrap sources"}
	}
	if len(node.Relay().PubSub().ListPeers(network.DefaultPubsubTopic)) > 0 {
		return network.BootstrapStatus{Joined: true, State: "ready"}
	}
	reason := current.Reason
	if reason == "" || current.State == "ready" {
		reason = "bootstrap relay path is not operational"
	}
	return network.BootstrapStatus{State: "degraded", Reason: reason}
}

func ProfileSnapshot(state ServiceState) network.Snapshot {
	profile := network.NormalizeProfile(state.ActiveProfile)
	snapshot := network.Snapshot{NodeProfile: state.NodeProfile, Profile: profile, Mode: state.ActiveMode}
	definition, err := network.ResolveProfile(profile)
	if err != nil {
		snapshot.Health = network.HealthStateFailed
		snapshot.SwitchReason = network.SwitchReasonStartupFailed
		snapshot.RecoveryState = network.RecoveryStateBlocked
		snapshot.ReducedFeatures = []network.TransportFeature{
			network.TransportFeatureRelay,
			network.TransportFeatureStore,
			network.TransportFeatureFilter,
			network.TransportFeatureLightpush,
		}
		return snapshot
	}
	assessment := runtimeAssessmentFor(state)
	snapshot.ActiveFamilies = cloneFamilies(definition.ActiveFamilies)
	snapshot.SuppressedFamilies = cloneFamilies(definition.SuppressedFamilies)
	snapshot.Health = assessment.health
	snapshot.SwitchReason = assessment.switchReason
	snapshot.SwitchAutomatic = assessment.switchAutomatic
	snapshot.RecoveryState = assessment.recoveryState
	snapshot.ReducedFeatures = cloneFeatures(assessment.reducedFeatures)
	snapshot.ActiveFeatures = activeMessagingFeatures(state)
	return snapshot
}

func BuildServicePartSnapshot(state ServiceState) ServicePartSnapshot {
	assessment := runtimeAssessmentFor(state)
	return ServicePartSnapshot{State: assessment.state, Reason: assessment.reason}
}

func HealthSnapshot(state ServiceState) network.HealthSignals {
	return network.HealthSignals{
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

func runtimeAssessmentFor(state ServiceState) runtimeAssessment {
	raw := baseRuntimeAssessment(state)
	applied := raw
	if state.SwitchReason != "" {
		applied.switchReason = state.SwitchReason
		applied.switchAutomatic = state.SwitchAutomatic
	}
	if state.RecoveryState != "" {
		applied.recoveryState = state.RecoveryState
	}
	if state.ActiveMode != network.ModeRestrictedDefense {
		return applied
	}
	if raw.health == network.HealthStateFailed || raw.health == network.HealthStateStopped || raw.health == network.HealthStateStarting {
		return applied
	}
	applied.state = "degraded"
	applied.health = network.HealthStateDegraded
	if applied.reason == "" {
		applied.reason = "restricted defense mode is active"
	}
	applied.reducedFeatures = appendUniqueFeatures(
		applied.reducedFeatures,
		network.TransportFeatureStore,
		network.TransportFeatureFilterService,
		network.TransportFeatureLightpushService,
		network.TransportFeatureSurfaceExpansion,
		network.TransportFeatureProfileRecoveryPending,
	)
	if applied.recoveryState == "" {
		applied.recoveryState = network.RecoveryStateRecoveryPending
	}
	return applied
}

func baseRuntimeAssessment(state ServiceState) runtimeAssessment {
	return classifyHealth(HealthSnapshot(state))
}

func classifyHealth(signals network.HealthSignals) runtimeAssessment {
	switch signals.ServiceState {
	case "failed":
		return failedAssessment(signals.ServiceReason)
	case "stopped":
		return runtimeAssessment{state: "stopped", health: network.HealthStateStopped, switchReason: network.SwitchReasonStopped}
	case "degraded":
		return degradedAssessment(signals)
	case "ready":
		return readyAssessment(signals)
	default:
		return runtimeAssessment{
			state:        signals.ServiceState,
			reason:       signals.ServiceReason,
			health:       network.HealthStateStarting,
			switchReason: network.SwitchReasonNotStarted,
		}
	}
}

func failedAssessment(reason string) runtimeAssessment {
	return runtimeAssessment{
		state:        "failed",
		reason:       reason,
		health:       network.HealthStateFailed,
		switchReason: network.SwitchReasonStartupFailed,
		reducedFeatures: []network.TransportFeature{
			network.TransportFeatureRelay,
			network.TransportFeatureStore,
			network.TransportFeatureFilter,
			network.TransportFeatureLightpush,
		},
		recoveryState: network.RecoveryStateBlocked,
	}
}

func degradedAssessment(signals network.HealthSignals) runtimeAssessment {
	reason := signals.BootstrapStatus.Reason
	if reason == "" {
		reason = signals.ServiceReason
	}
	return runtimeAssessment{
		state:           "degraded",
		reason:          reason,
		health:          network.HealthStateDegraded,
		switchReason:    network.SwitchReasonBootstrapDegraded,
		switchAutomatic: true,
		reducedFeatures: []network.TransportFeature{network.TransportFeaturePeerConnectivity, network.TransportFeatureBootstrapSync},
		recoveryState:   network.RecoveryStateRecoveryPending,
	}
}

func readyAssessment(signals network.HealthSignals) runtimeAssessment {
	if signals.EndpointCount == 0 || signals.UsableEndpointCount == 0 {
		if signals.Reachability.Mode == network.ReachabilityOutboundOnly {
			return readyWithBootstrap(signals)
		}
		if signals.Reachability.Mode == network.ReachabilityPublicDirect {
			reason := signals.Reachability.Reason
			if reason == "" {
				reason = "public ingress has not been verified"
			}
			return runtimeAssessment{
				state:           "degraded",
				reason:          reason,
				health:          network.HealthStateDegraded,
				switchReason:    network.SwitchReasonBootstrapDegraded,
				switchAutomatic: true,
				reducedFeatures: []network.TransportFeature{network.TransportFeatureInboundReachability, network.TransportFeatureEndpointPublication},
				recoveryState:   network.RecoveryStateRecoveryPending,
			}
		}
		return runtimeAssessment{
			state:           "degraded",
			reason:          "transport endpoints are not operational",
			health:          network.HealthStateDegraded,
			switchReason:    network.SwitchReasonBootstrapDegraded,
			switchAutomatic: true,
			reducedFeatures: []network.TransportFeature{network.TransportFeaturePeerConnectivity},
			recoveryState:   network.RecoveryStateRecoveryPending,
		}
	}
	return readyWithBootstrap(signals)
}

func readyWithBootstrap(signals network.HealthSignals) runtimeAssessment {
	if signals.NodeProfile == network.NodeProfileConstrainedClient && signals.BootstrapSourceCount == 0 {
		return runtimeAssessment{
			state: "degraded", reason: "constrained light client has no Filter, Lightpush, or Store providers",
			health: network.HealthStateDegraded, switchReason: network.SwitchReasonBootstrapDegraded, switchAutomatic: true,
			reducedFeatures: []network.TransportFeature{network.TransportFeatureFilter, network.TransportFeatureLightpush, network.TransportFeatureStoreRecovery}, recoveryState: network.RecoveryStateRecoveryPending,
		}
	}
	if signals.BootstrapSourceCount == 0 || signals.BootstrapStatus.State == "ready" {
		return runtimeAssessment{
			state:         "ready",
			health:        network.HealthStateReady,
			switchReason:  network.SwitchReasonStartupDefault,
			recoveryState: network.RecoveryStateSteady,
		}
	}
	reason := signals.BootstrapStatus.Reason
	if reason == "" {
		reason = "bootstrap relay path is not operational"
	}
	return runtimeAssessment{
		state:           "degraded",
		reason:          reason,
		health:          network.HealthStateDegraded,
		switchReason:    network.SwitchReasonBootstrapDegraded,
		switchAutomatic: true,
		reducedFeatures: []network.TransportFeature{network.TransportFeaturePeerConnectivity, network.TransportFeatureBootstrapSync},
		recoveryState:   network.RecoveryStateRecoveryPending,
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

func activeMessagingFeatures(state ServiceState) []network.TransportFeature {
	if state.State != "ready" && state.State != "degraded" {
		return nil
	}
	if state.NodeProfile != network.NodeProfileConstrainedClient {
		if state.ActiveMode == network.ModeRestrictedDefense {
			return []network.TransportFeature{network.TransportFeatureRelay}
		}
		return []network.TransportFeature{
			network.TransportFeatureRelay,
			network.TransportFeatureStore,
			network.TransportFeatureFilterService,
			network.TransportFeatureLightpushService,
		}
	}
	var active []network.TransportFeature
	if state.FilterPeerCount > 0 {
		active = append(active, network.TransportFeatureFilterClient)
	}
	if state.LightpushPeerCount > 0 {
		active = append(active, network.TransportFeatureLightpushClient)
	}
	if state.StorePeerCount > 0 {
		active = append(active, network.TransportFeatureStoreClient)
	}
	return active
}

func appendUniqueFeatures(base []network.TransportFeature, items ...network.TransportFeature) []network.TransportFeature {
	seen := make(map[string]struct{}, len(base)+len(items))
	out := make([]network.TransportFeature, 0, len(base)+len(items))
	for _, item := range append(cloneFeatures(base), items...) {
		if !item.Valid() {
			continue
		}
		if _, ok := seen[item.String()]; ok {
			continue
		}
		seen[item.String()] = struct{}{}
		out = append(out, item)
	}
	return out
}

func cloneFeatures(in []network.TransportFeature) []network.TransportFeature {
	return append([]network.TransportFeature(nil), in...)
}

func cloneFamilies(in []network.Family) []network.Family {
	return append([]network.Family(nil), in...)
}
