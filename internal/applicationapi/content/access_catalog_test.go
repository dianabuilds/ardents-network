package content_test

import (
	applicationcontent "ardents/internal/applicationapi/content"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"
	applicationv1connect "ardents/sdk/go/protocol/applicationv1/applicationv1connect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplicationContentCatalogueCoversEveryProtectedProcedureExactlyOnce(t *testing.T) {
	service := applicationv1.File_api_ardents_application_v1_content_proto.Services().ByName("ContentService")
	require.NotNil(t, service)

	procedures := applicationcontent.ProtectedProcedures()
	require.Len(t, procedures, service.Methods().Len())
	seen := make(map[string]int, len(procedures))
	for _, procedure := range procedures {
		seen[procedure]++
		rule, err := applicationcontent.RuleForProcedure(procedure)
		require.NoError(t, err)
		require.NotEmpty(t, rule.Action)
		require.NotEmpty(t, rule.ResourceKind)
	}
	for index := 0; index < service.Methods().Len(); index++ {
		method := service.Methods().Get(index)
		procedure := "/" + string(service.FullName()) + "/" + string(method.Name())
		require.Equal(t, 1, seen[procedure], "protected procedure %s must be catalogued exactly once", procedure)
	}
}

func TestApplicationContentCatalogueUsesFrozenActionsAndResources(t *testing.T) {
	tests := []struct {
		procedure string
		action    string
		kind      string
	}{
		{applicationv1connect.ContentServicePutProcedure, applicationcontent.ActionPut, "content-owner"},
		{applicationv1connect.ContentServiceGetProcedure, applicationcontent.ActionGet, "owned-content"},
	}
	for _, test := range tests {
		rule, err := applicationcontent.RuleForProcedure(test.procedure)
		require.NoError(t, err)
		require.Equal(t, test.action, rule.Action)
		require.Equal(t, test.kind, rule.ResourceKind)
		require.Equal(t, test.action == applicationcontent.ActionPut, rule.Mutating)
	}

	_, err := applicationcontent.RuleForProcedure("/ardents.application.v1.ContentService/Delete")
	require.ErrorIs(t, err, applicationcontent.ErrUnknownProcedure)
}
