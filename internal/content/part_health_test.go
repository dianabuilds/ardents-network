package content

import (
	"testing"
	"time"

	model "ardents/internal/content/catalog"

	"github.com/stretchr/testify/require"
)

func TestInventoryProjectsObservedLocalAndRelayTruth(t *testing.T) {
	local := testContentReference(t, "local")
	relay := testContentReference(t, "relay")
	remote := testContentReference(t, "remote")
	blobs := map[string]model.Blob{
		local.String(): {
			Reference: local,
			State:     "available-local",
		},
		relay.String(): {
			Reference: relay,
			State:     "retained-temporary",
			Retention: "relay-temporary",
			Encrypted: true,
		},
		remote.String(): {
			Reference: remote,
			State:     "available-remote",
		},
	}

	inv := ProjectInventory(2, 1, blobs, func(id string) (bool, int64) {
		switch id {
		case local.String():
			return true, 11
		case relay.String():
			return true, 17
		default:
			return false, 0
		}
	})

	require.Equal(t, 2, inv.Objects)
	require.Equal(t, 1, inv.Manifests)
	require.Equal(t, 3, inv.Blobs)
	require.Equal(t, 2, inv.LocalBlobs)
	require.Equal(t, 1, inv.RemoteBlobs)
	require.Equal(t, 1, inv.RetainedTemporary)
	require.Equal(t, 1, inv.RelayRetained)
	require.Equal(t, 1, inv.Encrypted)
	require.EqualValues(t, 28, inv.LocalBytes)
	require.EqualValues(t, 17, inv.RelayBytes)
}

func TestReconcileLoadedBlobsProjectsExpiredAndMissingPayloadState(t *testing.T) {
	now := time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC)
	removed := make([]string, 0)
	expired := testContentReference(t, "expired")
	missing := testContentReference(t, "missing")
	blobs := map[string]model.Blob{
		expired.String(): {
			Reference: expired,
			State:     "retained-temporary",
			ExpiresAt: now.Add(-time.Hour),
		},
		missing.String(): {
			Reference: missing,
			State:     "available-local",
		},
	}

	updated, changed, err := ReconcileLoadedBlobs(
		blobs,
		now,
		func(id string) bool { return id == expired.String() },
		func(id string) error {
			removed = append(removed, id)
			return nil
		},
	)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []string{expired.String()}, removed)
	require.Equal(t, "expired", updated[expired.String()].State)
	require.Equal(t, "deleted", updated[missing.String()].State)
	require.True(t, updated[missing.String()].ExpiresAt.IsZero())
	require.Equal(t, "retained-temporary", blobs[expired.String()].State)
	require.Equal(t, "available-local", blobs[missing.String()].State)
}
