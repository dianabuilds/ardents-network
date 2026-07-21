// Package configuration owns configuration protocol handlers and mappings.
// It does not own configuration semantics or runtime application.
package configuration

import (
	"context"
	"fmt"
	"net/http"

	runtimeconfig "ardents/internal/config"
	"ardents/internal/identity"
	localauth "ardents/internal/localapi/auth"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

type Controller struct {
	service runtimeconfig.Service
	auth    localauth.Config
}

func NewHandler(service runtimeconfig.Service, auth localauth.Config) *Controller {
	return &Controller{service: service, auth: auth}
}

func (h *Controller) GetEffectiveConfiguration(_ context.Context, req *connect.Request[protocol.GetEffectiveConfigurationRequest]) (*connect.Response[protocol.EffectiveConfigurationResponse], error) {
	if err := h.authorize(req.Header(), "config.effective", identity.AccessRead); err != nil {
		return nil, err
	}
	if h.service == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("operator configuration source is unavailable"))
	}
	return connect.NewResponse(&protocol.EffectiveConfigurationResponse{
		Status:        operationStatus("completed", "effective configuration available", true),
		Configuration: effectiveConfiguration(h.service.GetEffectiveConfig()),
	}), nil
}

func (h *Controller) ReloadConfiguration(ctx context.Context, req *connect.Request[protocol.ReloadConfigurationRequest]) (*connect.Response[protocol.ReloadConfigurationResponse], error) {
	if err := h.authorize(req.Header(), "config.reload", identity.AccessWrite); err != nil {
		return nil, err
	}
	if h.service == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("operator configuration source is unavailable"))
	}
	callCtx, cancel := rpc.MutationContext(ctx)
	defer cancel()
	result := h.service.ReloadConfig(callCtx)
	accepted := result.Outcome != runtimeconfig.OutcomeRejectedInvalid && result.Outcome != runtimeconfig.OutcomeRejectedImmutable && result.Outcome != runtimeconfig.OutcomeRolledBack
	return connect.NewResponse(&protocol.ReloadConfigurationResponse{
		Status:        operationStatus(string(result.Outcome), reloadReason(result), accepted),
		Result:        reloadConfiguration(result),
		Configuration: effectiveConfiguration(h.service.GetEffectiveConfig()),
	}), nil
}

func (h *Controller) authorize(header http.Header, operation string, access identity.Access) error {
	call, err := h.auth.CallContext(header)
	if err != nil {
		return err
	}
	return localauth.Require(call, operation, access)
}

func reloadReason(result runtimeconfig.ReloadResult) string {
	if result.Reason != "" {
		return result.Reason
	}
	return string(result.Outcome)
}
