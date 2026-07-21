package configuration

import (
	runtimeconfig "ardents/internal/config"
	protocol "ardents/internal/localapi/protocol"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func operationStatus(state, reason string, accepted bool) *protocol.OperationStatus {
	return &protocol.OperationStatus{State: state, Reason: reason, Accepted: accepted}
}

func effectiveConfiguration(in runtimeconfig.EffectiveSnapshot) *protocol.EffectiveConfigurationSnapshot {
	var loadedAt *timestamppb.Timestamp
	if !in.LoadedAt.IsZero() {
		loadedAt = timestamppb.New(in.LoadedAt)
	}
	var effective *structpb.Struct
	if len(in.Effective) > 0 {
		var err error
		effective, err = structpb.NewStruct(in.Effective)
		if err != nil {
			effective = nil
		}
	}
	return &protocol.EffectiveConfigurationSnapshot{
		ApiVersion: in.APIVersion, ActiveGeneration: in.ActiveGeneration,
		CandidateGeneration: in.CandidateGeneration, Fingerprint: in.Fingerprint,
		LoadedAt: loadedAt, Effective: effective,
		PendingRestart:    append([]string(nil), in.PendingRestart...),
		LastReloadOutcome: string(in.LastReload.Outcome), LastReloadReason: in.LastReload.Reason,
	}
}

func reloadConfiguration(in runtimeconfig.ReloadResult) *protocol.ConfigurationReloadResult {
	return &protocol.ConfigurationReloadResult{
		Outcome: string(in.Outcome), ActiveGeneration: in.ActiveGeneration,
		CandidateGeneration: in.CandidateGeneration,
		RestartRequired:     append([]string(nil), in.RestartRequired...),
		Immutable:           append([]string(nil), in.Immutable...), Reason: in.Reason,
	}
}
