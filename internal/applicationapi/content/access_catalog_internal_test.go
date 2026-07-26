package content

import (
	"testing"

	applicationv1 "ardents/sdk/go/protocol/applicationv1"

	"github.com/stretchr/testify/require"
)

func TestProtectedProcedureSetFailsClosedAgainstGeneratedContentService(t *testing.T) {
	service := applicationv1.File_api_ardents_application_v1_content_proto.Services().ByName("ContentService")
	require.NotNil(t, service)

	missing := cloneProcedureCatalogue(procedureAccess)
	delete(missing, "/ardents.application.v1.ContentService/Get")
	contracts, registrations, err := protectedProcedureSet(service, missing)
	require.Error(t, err)
	require.Nil(t, contracts)
	require.Nil(t, registrations)

	extra := cloneProcedureCatalogue(procedureAccess)
	extra["/ardents.application.v1.ContentService/Undeclared"] = ProcedureRule{
		Action: ActionGet, ResourceKind: "owned-content", OwnerRequired: true,
	}
	contracts, registrations, err = protectedProcedureSet(service, extra)
	require.Error(t, err)
	require.Nil(t, contracts)
	require.Nil(t, registrations)

	contracts, registrations, err = protectedProcedureSet(nil, procedureAccess)
	require.Error(t, err)
	require.Nil(t, contracts)
	require.Nil(t, registrations)
}

func cloneProcedureCatalogue(source map[string]ProcedureRule) map[string]ProcedureRule {
	result := make(map[string]ProcedureRule, len(source))
	for procedure, rule := range source {
		result[procedure] = rule
	}
	return result
}
