package content

import (
	"errors"
	"fmt"
	"sort"

	applicationadmission "ardents/internal/applicationapi/admission"
	identityaccess "ardents/internal/identity/access"
	identityprotocol "ardents/internal/identity/protocol"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"
	applicationv1connect "ardents/sdk/go/protocol/applicationv1/applicationv1connect"

	"google.golang.org/protobuf/reflect/protoreflect"
)

var ErrUnknownProcedure = errors.New("application content procedure is not registered")

type ProcedureRule struct {
	Action        string
	ResourceKind  string
	OwnerRequired bool
	Mutating      bool
}

var procedureAccess = map[string]ProcedureRule{
	applicationv1connect.ContentServicePutProcedure: {
		Action:        ActionPut,
		ResourceKind:  "content-owner",
		OwnerRequired: true,
		Mutating:      true,
	},
	applicationv1connect.ContentServiceGetProcedure: {
		Action:        ActionGet,
		ResourceKind:  "owned-content",
		OwnerRequired: true,
	},
}

func RuleForProcedure(procedure string) (ProcedureRule, error) {
	rule, ok := procedureAccess[procedure]
	if !ok {
		return ProcedureRule{}, ErrUnknownProcedure
	}
	return rule, nil
}

func ProtectedProcedures() []string {
	procedures := make([]string, 0, len(procedureAccess))
	for procedure := range procedureAccess {
		procedures = append(procedures, procedure)
	}
	sort.Strings(procedures)
	return procedures
}

func ProtectedProcedureSet() ([]applicationadmission.ProcedureContract, []applicationadmission.ProcedureRegistration, error) {
	service := applicationv1.File_api_ardents_application_v1_content_proto.Services().ByName("ContentService")
	return protectedProcedureSet(service, procedureAccess)
}

func protectedProcedureSet(service protoreflect.ServiceDescriptor, catalogue map[string]ProcedureRule) ([]applicationadmission.ProcedureContract, []applicationadmission.ProcedureRegistration, error) {
	if service == nil || len(catalogue) != service.Methods().Len() {
		return nil, nil, fmt.Errorf("application Content procedure registration is incomplete")
	}
	contracts := make([]applicationadmission.ProcedureContract, 0, service.Methods().Len())
	registrations := make([]applicationadmission.ProcedureRegistration, 0, service.Methods().Len())
	for index := 0; index < service.Methods().Len(); index++ {
		method := service.Methods().Get(index)
		procedure := "/" + string(service.FullName()) + "/" + string(method.Name())
		rule, registered := catalogue[procedure]
		if !registered {
			return nil, nil, fmt.Errorf("application Content procedure registration is incomplete")
		}
		contracts = append(contracts, applicationadmission.ProcedureContract{
			Procedure:     procedure,
			Action:        rule.Action,
			ResourceKind:  identityaccess.ResourceKind(rule.ResourceKind),
			OwnerRequired: rule.OwnerRequired,
			Mutating:      rule.Mutating,
		})
		registrations = append(registrations, applicationadmission.ProcedureRegistration{
			Procedure: procedure,
			Rule: applicationadmission.ProcedureRule{
				Action:        rule.Action,
				ResourceKind:  identityaccess.ResourceKind(rule.ResourceKind),
				OwnerRequired: rule.OwnerRequired,
				Mutating:      rule.Mutating,
				Resolve:       resolveProcedure(procedure),
				Finalize:      finalizeOwnedResource(rule.ResourceKind),
				MapTargetErr: func(err error) error {
					return mapTargetError(rule.Action, err)
				},
			},
		})
	}
	return contracts, registrations, nil
}

func resolveProcedure(procedure string) func(any) (identityaccess.ResourceTarget, error) {
	return func(message any) (identityaccess.ResourceTarget, error) {
		target, err := CanonicalizeResource(procedure, message)
		return identityaccess.ResourceTarget{Kind: identityaccess.ResourceKind(target.Kind), ID: target.ID}, err
	}
}

func finalizeOwnedResource(expectedKind string) identityaccess.ResourceFinalizer {
	return func(target identityaccess.ResourceTarget, audience identityaccess.Audience, _, effective string) (identityaccess.ResourceRef, error) {
		if audience.Interface != identityprotocol.Interface_INTERFACE_APPLICATION ||
			effective == "" || string(target.Kind) != expectedKind {
			return identityaccess.ResourceRef{}, identityaccess.ErrInvalidArgument
		}
		owner, err := identityaccess.ParseResourceOwner(effective)
		if err != nil || owner.IsNone() {
			return identityaccess.ResourceRef{}, identityaccess.ErrInvalidArgument
		}
		return identityaccess.NewResourceRef(audience.Node, owner, string(target.Kind), target.ID)
	}
}
