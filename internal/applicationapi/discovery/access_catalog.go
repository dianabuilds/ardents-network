package discovery

import (
	"fmt"

	applicationadmission "ardents/internal/applicationapi/admission"
	identityaccess "ardents/internal/identity/access"
	identityprotocol "ardents/internal/identity/protocol"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"
	applicationv1connect "ardents/sdk/go/protocol/applicationv1/applicationv1connect"
)

func ProtectedProcedureSet() ([]string, []applicationadmission.ProcedureRule, error) {
	service := applicationv1.File_api_ardents_application_v1_discovery_proto.Services().ByName("DiscoveryService")
	if service == nil || service.Methods().Len() != 1 {
		return nil, nil, fmt.Errorf("application Discovery procedure registration is incomplete")
	}
	method := service.Methods().Get(0)
	procedure := "/" + string(service.FullName()) + "/" + string(method.Name())
	if procedure != applicationv1connect.DiscoveryServiceResolveProcedure {
		return nil, nil, fmt.Errorf("application Discovery procedure registration is incomplete")
	}
	rule := applicationadmission.ProcedureRule{
		Procedure: procedure, Action: ActionResolve, ResourceKind: "service-type",
		OwnerRequired: false, Mutating: false,
		Resolve: func(message any) (identityaccess.ResourceTarget, error) {
			return CanonicalizeResource(procedure, message)
		},
		Finalize: func(target identityaccess.ResourceTarget, audience identityaccess.Audience, _, effective string) (identityaccess.ResourceRef, error) {
			if audience.Interface != identityprotocol.Interface_INTERFACE_APPLICATION ||
				effective == "" || target.Kind != "service-type" || !validCanonicalID(target.ID) {
				return identityaccess.ResourceRef{}, identityaccess.ErrInvalidArgument
			}
			return identityaccess.NewResourceRef(audience.Node, identityaccess.ResourceOwner{}, string(target.Kind), target.ID)
		},
		MapTargetErr: func(error) error {
			return invalidArgumentError()
		},
	}
	return []string{procedure}, []applicationadmission.ProcedureRule{rule}, nil
}
