package connectrpc

import (
	runtimeconfig "ardents/internal/runtime/config"
	ardents "ardents/proto/ardents/v1"
)

func effectiveConfigurationProto(in runtimeconfig.EffectiveSnapshot) *ardents.EffectiveConfigurationSnapshot {
	return &ardents.EffectiveConfigurationSnapshot{
		ApiVersion: in.APIVersion, ActiveGeneration: in.ActiveGeneration,
		CandidateGeneration: in.CandidateGeneration, Fingerprint: in.Fingerprint,
		LoadedAt: ts(in.LoadedAt), Effective: toStruct(in.Effective),
		PendingRestart:    append([]string(nil), in.PendingRestart...),
		LastReloadOutcome: string(in.LastReload.Outcome), LastReloadReason: in.LastReload.Reason,
	}
}

func reloadConfigurationProto(in runtimeconfig.ReloadResult) *ardents.ConfigurationReloadResult {
	return &ardents.ConfigurationReloadResult{
		Outcome: string(in.Outcome), ActiveGeneration: in.ActiveGeneration,
		CandidateGeneration: in.CandidateGeneration,
		RestartRequired:     append([]string(nil), in.RestartRequired...),
		Immutable:           append([]string(nil), in.Immutable...), Reason: in.Reason,
	}
}
