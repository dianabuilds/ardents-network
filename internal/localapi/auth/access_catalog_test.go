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
		ardentsv1.File_api_ardents_v1_authority_proto.Services().Get(0),
		ardentsv1.File_api_ardents_v1_authority_proto.Services().Get(1),
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
			if service.Name() == "AuthorityService" || service.Name() == "ChannelDeliveryService" || service.Name() == "NodeService" || service.Name() == "ConfigurationService" || service.Name() == "NetworkService" || service.Name() == "WorkloadService" || service.Name() == "ContentService" || service.Name() == "TransferService" || service.Name() == "RetentionService" || service.Name() == "DiagnosticsService" {
				require.NotEmpty(t, rule.ResourceKind, procedure)
			}
			prior, duplicate := actions[rule.Action]
			require.Falsef(t, duplicate, "action %q is shared by %s and %s", rule.Action, prior, procedure)
			actions[rule.Action] = procedure
		}
	}
}

func TestNodeFeaturesActionIsExactAndOldActionIsAbsent(t *testing.T) {
	rule, ok := RuleForProcedure("/ardents.v1.NodeService/GetNodeFeatures")
	require.True(t, ok)
	require.Equal(t, "node.features", rule.Action)
	require.NotContains(t, OperatorActions(), "node.capabilities")
	_, ok = RuleForProcedure("/ardents.v1.NodeService/GetNodeCapabilities")
	require.False(t, ok)
}

func TestProcedureMutationClassificationIsServerOwned(t *testing.T) {
	for procedure, want := range map[string]bool{
		"/ardents.v1.AuthorityService/CreateRealmAuthority":  true,
		"/ardents.v1.AuthorityService/InspectRealmAuthority": false,
		"/ardents.v1.NodeService/StartNode":                  true,
		"/ardents.v1.NodeService/GetNodeStatus":              false,
		"/ardents.v1.DiagnosticsService/ListRecentEvents":    false,
		"/ardents.v1.ContentService/PublishBlob":             true,
		"/ardents.v1.ContentService/GetBlob":                 false,
	} {
		rule, ok := RuleForProcedure(procedure)
		require.True(t, ok, procedure)
		require.Equal(t, want, rule.Mutating, procedure)
	}
}
