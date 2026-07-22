package configuration

import (
	"context"
	"testing"
	"time"

	runtimeconfig "ardents/internal/config"
	"github.com/stretchr/testify/require"
)

type configurationStub struct {
	snapshot runtimeconfig.EffectiveSnapshot
	result   runtimeconfig.ReloadResult
}

func (s *configurationStub) GetEffectiveConfig() runtimeconfig.EffectiveSnapshot { return s.snapshot }

func (s *configurationStub) ReloadConfig(context.Context) runtimeconfig.ReloadResult { return s.result }

func TestConfigurationSurfaceReturnsRedactedEffectiveSnapshot(t *testing.T) {
	stub := &configurationStub{snapshot: runtimeconfig.EffectiveSnapshot{
		APIVersion: runtimeconfig.Version, ActiveGeneration: 2, CandidateGeneration: 3,
		LoadedAt: time.Now().UTC(), Effective: map[string]any{
			"api": map[string]any{"token_file": "configured"},
		}, PendingRestart: []string{"network.listen_port"},
	}}
	server := NewHandler(stub)
	response, rpcErr := server.effectiveConfiguration()
	require.Nil(t, rpcErr)
	require.Equal(t, uint64(2), response.Configuration.ActiveGeneration)
	require.Equal(t, "configured", response.Configuration.Effective.Fields["api"].GetStructValue().Fields["token_file"].GetStringValue())
	require.Equal(t, []string{"network.listen_port"}, response.Configuration.PendingRestart)
}

func TestConfigurationReloadRequiresWriteAndReturnsOutcome(t *testing.T) {
	stub := &configurationStub{
		snapshot: runtimeconfig.EffectiveSnapshot{APIVersion: runtimeconfig.Version},
		result:   runtimeconfig.ReloadResult{Outcome: runtimeconfig.OutcomeApplied, ActiveGeneration: 2, CandidateGeneration: 2},
	}
	server := NewHandler(stub)
	response, rpcErr := server.reloadConfiguration(context.Background())
	require.Nil(t, rpcErr)
	require.Equal(t, "applied", response.Result.Outcome)
	require.True(t, response.Status.Accepted)
}
