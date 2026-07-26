package discovery

import (
	"errors"

	requestvalidation "ardents/internal/applicationapi/requestvalidation"
	identityaccess "ardents/internal/identity/access"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"
	applicationv1connect "ardents/sdk/go/protocol/applicationv1/applicationv1connect"
)

var ErrUnknownProcedure = errors.New("application discovery procedure is not registered")

func CanonicalizeResource(procedure string, message any) (identityaccess.ResourceTarget, error) {
	if procedure != applicationv1connect.DiscoveryServiceResolveProcedure {
		return identityaccess.ResourceTarget{}, ErrUnknownProcedure
	}
	request, ok := message.(*applicationv1.ResolveServiceRequest)
	if !ok || request == nil || !request.ProtoReflect().IsValid() || requestvalidation.HasUnknownFields(request) {
		return identityaccess.ResourceTarget{}, ErrInvalidArgument
	}
	query := Query{
		ServiceType: request.GetServiceType(), AcceptedSchemes: append([]string(nil), request.GetAcceptedSchemes()...),
	}
	if !validQuery(query) {
		return identityaccess.ResourceTarget{}, ErrInvalidArgument
	}
	return identityaccess.ResourceTarget{Kind: "service-type", ID: query.ServiceType}, nil
}
