package content

import (
	"testing"
	"time"

	model "ardents/internal/content/catalog"

	"github.com/stretchr/testify/require"
)

func TestInventoryProjectsObservedLocalAndRelayTruth(t *testing.T) {
	blobs := map[string]model.Blob{
		"local": {
			ID:    "local",
			State: "available-local",
		},
		"relay": {
			ID:        "relay",
			State:     "retained-temporary",
			Retention: "relay-temporary",
			Encrypted: true,
		},
		"remote": {
			ID:    "remote",
			State: "available-remote",
		},
	}

	inv := ProjectInventory(2, 1, blobs, func(id string) (bool, int64) {
		switch id {
		case "local":
			return true, 11
		case "relay":
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
	blobs := map[string]model.Blob{
		"expired": {
			ID:        "expired",
			State:     "retained-temporary",
			ExpiresAt: now.Add(-time.Hour),
		},
		"missing": {
			ID:    "missing",
			State: "available-local",
		},
	}

	updated, changed, err := ReconcileLoadedBlobs(
		blobs,
		now,
		func(id string) bool { return id == "expired" },
		func(id string) error {
			removed = append(removed, id)
			return nil
		},
	)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []string{"expired"}, removed)
	require.Equal(t, "expired", updated["expired"].State)
	require.Equal(t, "deleted", updated["missing"].State)
	require.True(t, updated["missing"].ExpiresAt.IsZero())
	require.Equal(t, "retained-temporary", blobs["expired"].State)
	require.Equal(t, "available-local", blobs["missing"].State)
}
