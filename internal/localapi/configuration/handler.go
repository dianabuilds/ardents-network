// Package configuration owns configuration protocol handlers and mappings.
// It does not own configuration semantics or runtime application.
package configuration

import (
	"context"
	"fmt"

	runtimeconfig "ardents/internal/config"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

type Controller struct {
	service runtimeconfig.Service
}

func NewHandler(service runtimeconfig.Service) *Controller {
	return &Controller{service: service}
}

func (h *Controller) GetEffectiveConfiguration(ctx context.Context, _ *connect.Request[protocol.GetEffectiveConfigurationRequest]) (*connect.Response[protocol.EffectiveConfigurationResponse], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*protocol.EffectiveConfigurationResponse, *rpc.Error) {
		return h.effectiveConfiguration()
	})
}

func (h *Controller) ReloadConfiguration(ctx context.Context, req *connect.Request[protocol.ReloadConfigurationRequest]) (*connect.Response[protocol.ReloadConfigurationResponse], error) {
	callCtx, cancel := rpc.MutationContext(ctx)
	defer cancel()
	return rpc.RespondContext(ctx, func(rpc.Call) (*protocol.ReloadConfigurationResponse, *rpc.Error) {
		return h.reloadConfiguration(callCtx)
	})
}

func (h *Controller) effectiveConfiguration() (*protocol.EffectiveConfigurationResponse, *rpc.Error) {
	if h.service == nil {
		return nil, rpc.MapError("config", "config.effective", "unavailable", "operator configuration source is unavailable", true, fmt.Errorf("configuration unavailable"))
	}
	return &protocol.EffectiveConfigurationResponse{Status: operationStatus("completed", "effective configuration available", true), Configuration: effectiveConfiguration(h.service.GetEffectiveConfig())}, nil
}

func (h *Controller) reloadConfiguration(ctx context.Context) (*protocol.ReloadConfigurationResponse, *rpc.Error) {
	if h.service == nil {
		return nil, rpc.MapError("config", "config.reload", "unavailable", "operator configuration source is unavailable", true, fmt.Errorf("configuration unavailable"))
	}
	result := h.service.ReloadConfig(ctx)
	accepted := result.Outcome != runtimeconfig.OutcomeRejectedInvalid && result.Outcome != runtimeconfig.OutcomeRejectedImmutable && result.Outcome != runtimeconfig.OutcomeRolledBack
	return &protocol.ReloadConfigurationResponse{Status: operationStatus(string(result.Outcome), reloadReason(result), accepted), Result: reloadConfiguration(result), Configuration: effectiveConfiguration(h.service.GetEffectiveConfig())}, nil
}

func reloadReason(result runtimeconfig.ReloadResult) string {
	if result.Reason != "" {
		return result.Reason
	}
	return string(result.Outcome)
}
