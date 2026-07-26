package discovery_test

import (
	"testing"

	applicationv1 "ardents/sdk/go/protocol/applicationv1"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestDiscoveryProtocolIsAdditiveAndPubliclyBounded(t *testing.T) {
	service := applicationv1.File_api_ardents_application_v1_discovery_proto.Services().ByName("DiscoveryService")
	require.NotNil(t, service)
	require.Equal(t, 1, service.Methods().Len())
	require.Equal(t, protoreflect.Name("Resolve"), service.Methods().Get(0).Name())

	request := (&applicationv1.ResolveServiceRequest{}).ProtoReflect().Descriptor()
	require.Equal(t, protoreflect.FieldNumber(1), request.Fields().ByName("service_type").Number())
	require.Equal(t, protoreflect.FieldNumber(2), request.Fields().ByName("accepted_schemes").Number())

	target := (&applicationv1.ResolvedServiceTarget{}).ProtoReflect().Descriptor()
	require.Equal(t, 3, target.Fields().Len())
	require.Equal(t, protoreflect.FieldNumber(1), target.Fields().ByName("service_id").Number())
	require.Equal(t, protoreflect.FieldNumber(2), target.Fields().ByName("endpoint").Number())
	require.Equal(t, protoreflect.FieldNumber(3), target.Fields().ByName("scheme").Number())

	response := (&applicationv1.ResolveServiceResponse{}).ProtoReflect().Descriptor()
	require.Equal(t, 1, response.Fields().Len())
	require.Equal(t, protoreflect.FieldNumber(1), response.Fields().ByName("targets").Number())
}
