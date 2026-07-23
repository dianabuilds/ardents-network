package workload

import (
	"testing"

	protocol "ardents/internal/localapi/protocol"
	workloadapi "ardents/internal/workload"
	workloadregistry "ardents/internal/workload/registry"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestWorkloadRequirementWireMappingIsTypedAndRejectsMalformedInput(t *testing.T) {
	mapped, err := fromWorkloadSpecSnapshot(&protocol.WorkloadSpecSnapshot{
		Id: "work.gpu", Kind: "service", Owner: "node",
		Requirements: []string{"gpu", "network.read"},
	})
	require.NoError(t, err)
	require.Equal(t, []workloadregistry.WorkloadRequirement{"gpu", "network.read"}, mapped.Requirements)
	projected, err := toWorkloadSpecSnapshot(mapped)
	require.NoError(t, err)
	require.Equal(t, []string{"gpu", "network.read"}, projected.GetRequirements())

	_, err = fromWorkloadSpecSnapshot(&protocol.WorkloadSpecSnapshot{
		Id: "work.gpu", Kind: "service", Owner: "node", Requirements: []string{" GPU "},
	})
	require.Error(t, err)
}

func TestWorkloadRequirementWireProjectionRejectsInvalidTypedValue(t *testing.T) {
	_, err := toWorkloadSpecSnapshot(workloadapi.SpecSnapshot{
		ID: "work.bad", Kind: "service", Owner: "node",
		Requirements: []workloadregistry.WorkloadRequirement{" GPU "},
	})
	require.Error(t, err)
}

func TestWorkloadRequirementWireRejectsReservedCapabilitiesTag(t *testing.T) {
	wire := []byte{0x3a, 0x03, 'g', 'p', 'u'} // removed repeated string field 7
	var obsolete protocol.WorkloadSpecSnapshot
	require.NoError(t, proto.Unmarshal(wire, &obsolete))
	require.NotEmpty(t, obsolete.ProtoReflect().GetUnknown())

	_, err := fromWorkloadSpecSnapshot(&obsolete)
	require.Error(t, err)

	descriptor := obsolete.ProtoReflect().Descriptor()
	require.Nil(t, descriptor.Fields().ByNumber(7))
	require.Equal(t, "requirements", string(descriptor.Fields().ByNumber(10).Name()))
}
