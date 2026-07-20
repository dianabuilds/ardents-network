package connectrpc

import (
	"context"

	runtimeconfig "ardents/internal/runtime/config"
	ardents "ardents/proto/ardents/v1"

	"connectrpc.com/connect"
)

func (s *Server) GetEffectiveConfiguration(
	ctx context.Context,
	req *connect.Request[ardents.GetEffectiveConfigurationRequest],
) (*connect.Response[ardents.EffectiveConfigurationResponse], error) {
	return respond(s, req.Header(), func(call callContext) (*ardents.EffectiveConfigurationResponse, *rpcError) {
		if err := requireRead(call, "config", "config.effective"); err != nil {
			return nil, err
		}
		if s.configuration == nil {
			return nil, unavailableConfigError("config.effective")
		}
		return &ardents.EffectiveConfigurationResponse{
			Status:        statusProto("completed", "effective configuration available", true),
			Configuration: effectiveConfigurationProto(s.configuration.GetEffectiveConfig()),
		}, nil
	})
}

func (s *Server) ReloadConfiguration(
	ctx context.Context,
	req *connect.Request[ardents.ReloadConfigurationRequest],
) (*connect.Response[ardents.ReloadConfigurationResponse], error) {
	callCtx, cancel := mutationContext(ctx)
	defer cancel()
	return respond(s, req.Header(), func(call callContext) (*ardents.ReloadConfigurationResponse, *rpcError) {
		if err := requireWrite(call, "config", "config.reload"); err != nil {
			return nil, err
		}
		if s.configuration == nil {
			return nil, unavailableConfigError("config.reload")
		}
		result := s.configuration.ReloadConfig(callCtx)
		accepted := result.Outcome != runtimeconfig.OutcomeRejectedInvalid &&
			result.Outcome != runtimeconfig.OutcomeRejectedImmutable && result.Outcome != runtimeconfig.OutcomeRolledBack
		return &ardents.ReloadConfigurationResponse{
			Status:        statusProto(string(result.Outcome), reloadStatusReason(result), accepted),
			Result:        reloadConfigurationProto(result),
			Configuration: effectiveConfigurationProto(s.configuration.GetEffectiveConfig()),
		}, nil
	})
}

func unavailableConfigError(operation string) *rpcError {
	return mapAPIError("config", operation, "configuration_unavailable",
		"operator configuration source is unavailable", false, nil)
}

func reloadStatusReason(result runtimeconfig.ReloadResult) string {
	if result.Reason != "" {
		return result.Reason
	}
	return string(result.Outcome)
}
