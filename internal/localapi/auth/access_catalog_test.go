package auth

import (
	"fmt"
	"testing"

	ardentsv1 "ardents/internal/localapi/protocol"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestProcedureAccessCatalogCoversEveryRPCExactlyOnce(t *testing.T) {
	services := []protoreflect.ServiceDescriptor{
		ardentsv1.File_api_ardents_v1_node_proto.Services().Get(0),
		ardentsv1.File_api_ardents_v1_configuration_proto.Services().Get(0),
		ardentsv1.File_api_ardents_v1_network_proto.Services().Get(0),
		ardentsv1.File_api_ardents_v1_workload_proto.Services().Get(0),
		ardentsv1.File_api_ardents_v1_content_proto.Services().Get(0),
		ardentsv1.File_api_ardents_v1_transfer_proto.Services().Get(0),
		ardentsv1.File_api_ardents_v1_retention_proto.Services().Get(0),
		ardentsv1.File_api_ardents_v1_diagnostics_proto.Services().Get(0),
	}
	methodCount := 0
	for _, service := range services {
		methodCount += service.Methods().Len()
	}
	require.Equal(t, methodCount, len(procedureAccess))

	actions := make(map[string]string, len(procedureAccess))
	for _, service := range services {
		for index := 0; index < service.Methods().Len(); index++ {
			method := service.Methods().Get(index)
			procedure := fmt.Sprintf("/%s/%s", service.FullName(), method.Name())
			rule, ok := procedureAccess[procedure]
			require.True(t, ok, procedure)
			require.NotEmpty(t, rule.Action, procedure)
			require.NotEmpty(t, rule.Domain, procedure)
			require.NotEmpty(t, rule.Access, procedure)
			prior, duplicate := actions[rule.Action]
			require.Falsef(t, duplicate, "action %q is shared by %s and %s", rule.Action, prior, procedure)
			actions[rule.Action] = procedure
		}
	}
}
