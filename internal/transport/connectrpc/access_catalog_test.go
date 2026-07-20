package connectrpc

import (
	"fmt"
	"testing"

	ardentsv1 "ardents/proto/ardents/v1"

	"github.com/stretchr/testify/require"
)

func TestProcedureAccessCatalogCoversEveryRPCExactlyOnce(t *testing.T) {
	service := ardentsv1.File_proto_ardents_v1_ardents_proto.Services().ByName("ArdentsService")
	require.NotNil(t, service)
	require.Equal(t, service.Methods().Len(), len(procedureAccess))

	actions := make(map[string]string, len(procedureAccess))
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
