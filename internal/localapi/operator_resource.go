package localapi

import (
	"strings"

	identityaccess "ardents/internal/identity/access"
	localauth "ardents/internal/localapi/auth"
	authorityhandler "ardents/internal/localapi/authority"
	channeldeliveryhandler "ardents/internal/localapi/channeldelivery"
	contenthandler "ardents/internal/localapi/content"
	diagnosticshandler "ardents/internal/localapi/diagnostics"
	networkhandler "ardents/internal/localapi/network"
	transferhandler "ardents/internal/localapi/transfer"
	workloadhandler "ardents/internal/localapi/workload"
)

func CanonicalizeOperatorResource(procedure string, message any) (identityaccess.ResourceTarget, error) {
	rule, known := localauth.RuleForProcedure(procedure)
	if !known || rule.ResourceKind == "" {
		return identityaccess.ResourceTarget{}, localauth.ErrUnknownProcedure
	}
	kind := identityaccess.ResourceKind(rule.ResourceKind)
	switch {
	case strings.HasPrefix(procedure, "/ardents.v1.AuthorityService/"):
		return authorityhandler.CanonicalizeResource(procedure, message, string(kind))
	case strings.HasPrefix(procedure, "/ardents.v1.ChannelDeliveryService/"):
		return channeldeliveryhandler.CanonicalizeResource(procedure, message, string(kind))
	case strings.HasPrefix(procedure, "/ardents.v1.NetworkService/"):
		return networkhandler.CanonicalizeResource(procedure, message, kind)
	case strings.HasPrefix(procedure, "/ardents.v1.DiagnosticsService/"):
		return diagnosticshandler.CanonicalizeResource(procedure, message, kind)
	case strings.HasPrefix(procedure, "/ardents.v1.WorkloadService/"):
		return workloadhandler.CanonicalizeResource(procedure, message, kind)
	case strings.HasPrefix(procedure, "/ardents.v1.ContentService/"), strings.HasPrefix(procedure, "/ardents.v1.RetentionService/"):
		return contenthandler.CanonicalizeResource(procedure, message, kind)
	case strings.HasPrefix(procedure, "/ardents.v1.TransferService/"):
		return transferhandler.CanonicalizeResource(procedure, message, kind)
	default:
		return localauth.CanonicalizeCoreResource(procedure, message)
	}
}
