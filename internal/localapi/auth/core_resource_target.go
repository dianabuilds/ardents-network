package auth

import (
	"errors"

	identityaccess "ardents/internal/identity/access"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"
)

var ErrInvalidResourceTarget = errors.New("operator resource target is invalid")

func CanonicalizeCoreResource(procedure string, message any) (identityaccess.ResourceTarget, error) {
	rule, known := RuleForProcedure(procedure)
	if !known || rule.ResourceKind == "" {
		return identityaccess.ResourceTarget{}, ErrUnknownProcedure
	}
	target := identityaccess.ResourceTarget{Kind: identityaccess.ResourceKind(rule.ResourceKind)}
	switch procedure {
	case ardentsv1connect.NodeServiceStartNodeProcedure:
		_, known = message.(*protocol.StartNodeRequest)
	case ardentsv1connect.NodeServiceStopNodeProcedure:
		_, known = message.(*protocol.StopNodeRequest)
	case ardentsv1connect.NodeServiceGetNodeStatusProcedure:
		_, known = message.(*protocol.GetNodeStatusRequest)
	case ardentsv1connect.NodeServiceGetNodeCapabilitiesProcedure:
		_, known = message.(*protocol.GetNodeCapabilitiesRequest)
	case ardentsv1connect.NodeServiceGetNodeRuntimeProcedure:
		_, known = message.(*protocol.GetNodeRuntimeRequest)
	case ardentsv1connect.NodeServiceStreamNodeEventsProcedure:
		_, known = message.(*protocol.StreamNodeEventsRequest)
	case ardentsv1connect.ConfigurationServiceGetEffectiveConfigurationProcedure:
		_, known = message.(*protocol.GetEffectiveConfigurationRequest)
	case ardentsv1connect.ConfigurationServiceReloadConfigurationProcedure:
		_, known = message.(*protocol.ReloadConfigurationRequest)
	default:
		return identityaccess.ResourceTarget{}, ErrUnknownProcedure
	}
	if !known {
		return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
	}
	return target, nil
}
