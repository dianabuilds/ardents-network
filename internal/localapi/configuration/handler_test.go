package configuration

import (
	"context"
	"testing"
	"time"

	runtimeconfig "ardents/internal/config"
	localauth "ardents/internal/localapi/auth"
	ardents "ardents/internal/localapi/protocol"

	"connectrpc.com/connect"
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
	server := NewHandler(stub, configAuth("config.effective"))
	req := connect.NewRequest(&ardents.GetEffectiveConfigurationRequest{})
	req.Header().Set("Authorization", "Bearer token")

	response, err := server.GetEffectiveConfiguration(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, uint64(2), response.Msg.Configuration.ActiveGeneration)
	require.Equal(t, "configured", response.Msg.Configuration.Effective.Fields["api"].GetStructValue().Fields["token_file"].GetStringValue())
	require.Equal(t, []string{"network.listen_port"}, response.Msg.Configuration.PendingRestart)
}

func TestConfigurationReloadRequiresWriteAndReturnsOutcome(t *testing.T) {
	stub := &configurationStub{
		snapshot: runtimeconfig.EffectiveSnapshot{APIVersion: runtimeconfig.Version},
		result:   runtimeconfig.ReloadResult{Outcome: runtimeconfig.OutcomeApplied, ActiveGeneration: 2, CandidateGeneration: 2},
	}
	server := NewHandler(stub, configAuth("config.reload"))
	req := connect.NewRequest(&ardents.ReloadConfigurationRequest{})
	req.Header().Set("Authorization", "Bearer token")

	response, err := server.ReloadConfiguration(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, "applied", response.Msg.Result.Outcome)
	require.True(t, response.Msg.Status.Accepted)

	server.auth = configAuth("config.effective")
	_, err = server.ReloadConfiguration(context.Background(), req)
	require.Error(t, err)
}

func configAuth(capability string) localauth.Config {
	return localauth.Config{Token: "token", SubjectID: "operator", Capabilities: []string{capability}}
}
