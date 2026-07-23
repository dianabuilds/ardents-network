package node

import (
	"testing"

	ardentsv1 "ardents/internal/localapi/protocol"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestNodeFeaturesWireContractHasNoCapabilitiesAlias(t *testing.T) {
	nodeService := ardentsv1.File_api_ardents_v1_node_proto.Services().ByName("NodeService")
	require.NotNil(t, nodeService.Methods().ByName("GetNodeFeatures"))
	require.Nil(t, nodeService.Methods().ByName("GetNodeCapabilities"))

	messages := ardentsv1.File_api_ardents_v1_types_proto.Messages()
	require.NotNil(t, messages.ByName("NodeFeaturesSnapshot"))
	require.NotNil(t, messages.ByName("NodeFeaturesResponse"))
	require.NotNil(t, messages.ByName("GetNodeFeaturesRequest"))
	require.Nil(t, messages.ByName("CapabilitiesSnapshot"))
	require.Nil(t, messages.ByName("CapabilitiesResponse"))
	require.Nil(t, messages.ByName("GetNodeCapabilitiesRequest"))

	assertReplacedField(t, messages.ByName("TransportSnapshot"), "reduced_capabilities", 7, "reduced_features", 11)
	assertReplacedField(t, messages.ByName("TransportSnapshot"), "active_capabilities", 10, "active_features", 12)
	assertReplacedField(t, messages.ByName("NetworkStatusSnapshot"), "reduced_capabilities", 7, "reduced_features", 26)
	assertReplacedField(t, messages.ByName("NetworkStatusSnapshot"), "active_capabilities", 19, "active_features", 27)
	assertReplacedField(t, messages.ByName("NodeStatusResponse"), "capabilities", 3, "features", 4)
	assertReplacedField(t, messages.ByName("NodeFeaturesResponse"), "capabilities", 1, "features", 2)
}

func assertReplacedField(t *testing.T, message protoreflect.MessageDescriptor, oldName protoreflect.Name, oldTag protoreflect.FieldNumber, newName protoreflect.Name, newTag protoreflect.FieldNumber) {
	t.Helper()
	require.NotNil(t, message)
	require.Nil(t, message.Fields().ByName(oldName))
	require.True(t, message.ReservedNames().Has(oldName))
	require.True(t, message.ReservedRanges().Has(oldTag))
	field := message.Fields().ByName(newName)
	require.NotNil(t, field)
	require.Equal(t, newTag, field.Number())
}
