package node

import (
	"testing"
	"time"

	daemonruntime "ardents/internal/daemon"
	diagapi "ardents/internal/diagnostics"
	ardentsv1 "ardents/internal/localapi/protocol"
	"ardents/internal/network"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestIdentitySnapshotDoesNotProjectFakeNodeDevice(t *testing.T) {
	fields := (&ardentsv1.IdentitySnapshot{}).ProtoReflect().Descriptor().Fields()
	require.Nil(t, fields.ByName(protoreflect.Name("device")))
}

func TestSnapshotMappingRejectsUnknownTransportFeature(t *testing.T) {
	_, err := toSnapshot(daemonruntime.SystemSnapshot{Transport: &network.Snapshot{
		ActiveFeatures: []network.TransportFeature{"secret_unknown_feature"},
	}})

	require.EqualError(t, err, "invalid transport feature")
}

func TestSurfaceMappersPreserveNewSnapshotFields(t *testing.T) {
	now := time.Now().UTC()
	runtime := toNodeRuntimeSnapshot(daemonruntime.RuntimeSnapshot{
		Health: diagapi.HealthSnapshot{
			State:                  "degraded",
			PrimaryReason:          &diagapi.ReasonSnapshot{Code: "node.transport.degraded", Summary: "transport degraded"},
			Subsystems:             []diagapi.SubsystemHealthSnapshot{{Domain: "transport", State: "degraded"}},
			OperatorActionRequired: true,
			UpdatedAt:              now,
		},
	})
	health := runtime.GetHealth()
	require.Equal(t, "degraded", health.GetState())
	require.True(t, health.GetOperatorActionRequired())
	require.NotNil(t, health.GetPrimaryReason())
	require.Equal(t, now.Unix(), health.GetUpdatedAt().AsTime().Unix())
	require.Len(t, health.GetSubsystems(), 1)
	require.Equal(t, "transport", health.GetSubsystems()[0].GetDomain())

}
