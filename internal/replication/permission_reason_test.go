package replication

import (
	"errors"
	"testing"

	"ardents/internal/replication/placement"

	"github.com/stretchr/testify/require"
)

func TestPermissionReasonRejectsCapabilityAlias(t *testing.T) {
	require.Equal(t, placement.ReasonPermission, safeReason(errors.New("replica permission is missing")))
	require.Equal(t, reasonReplicaControlRejected, safeReason(errors.New("replica capability is missing")))
	require.True(t, validCapacityDenial(placement.ReasonPermission))
	require.False(t, validCapacityDenial("capability_denied"))
}
