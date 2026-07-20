package replication

import (
	"testing"

	appdata "ardents/internal/data"
	"ardents/internal/data/placement"

	"github.com/stretchr/testify/require"
)

func TestBatchDueRepairsSerializesOneBlobIntentAndPreservesCrossBlobParallelism(t *testing.T) {
	batches := batchDueRepairs([]appdata.RepairRecord{
		{ID: "repair-c", IntentVersion: 1, BlobID: "blob-a", MissingOrdinal: 2},
		{ID: "repair-a", IntentVersion: 1, BlobID: "blob-a", MissingOrdinal: 1},
		{ID: "repair-b", IntentVersion: 1, BlobID: "blob-b", MissingOrdinal: 1},
		{ID: "repair-d", IntentVersion: 2, BlobID: "blob-a", MissingOrdinal: 1},
	})

	require.Len(t, batches, 3)
	require.Equal(t, "blob-a", batches[0].blobID)
	require.Equal(t, uint64(1), batches[0].intentVersion)
	require.Equal(t, []string{"repair-a", "repair-c"}, repairRecordIDs(batches[0].repairs))
	require.Equal(t, "blob-b", batches[1].blobID)
	require.Equal(t, uint64(2), batches[2].intentVersion)
}

func repairRecordIDs(repairs []appdata.RepairRecord) []string {
	ids := make([]string, 0, len(repairs))
	for _, repair := range repairs {
		ids = append(ids, repair.ID)
	}
	return ids
}

func TestPlacementUnsatisfiedErrorAggregatesReasonsWithoutPeerIdentity(t *testing.T) {
	err := placementUnsatisfiedError(placement.SelectionDecision{Denials: []placement.Denial{
		{NodeID: "peer-secret-one", Reason: placement.ReasonQuota},
		{NodeID: "peer-secret-two", Reason: placement.ReasonQuota},
		{NodeID: "peer-secret-three", Reason: placement.ReasonObservation},
	}}, 0, 1)

	require.ErrorContains(t, err, "quota_refused:2")
	require.ErrorContains(t, err, "observation_unavailable:1")
	require.NotContains(t, err.Error(), "peer-secret")
}
