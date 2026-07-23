package network

import (
	"testing"

	ardentsv1 "ardents/internal/localapi/protocol"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestDiscoveryRecordWireUsesVersionedKindSpecificFacts(t *testing.T) {
	fields := (&ardentsv1.DiscoveryRecord{}).ProtoReflect().Descriptor().Fields()
	require.NotNil(t, fields.ByName(protoreflect.Name("version")))
	require.NotNil(t, fields.ByName(protoreflect.Name("node_facts")))
	require.NotNil(t, fields.ByName(protoreflect.Name("service_facts")))
	for _, legacy := range []string{"id", "kind", "subject", "device", "owner"} {
		require.Nil(t, fields.ByName(protoreflect.Name(legacy)))
	}
}
