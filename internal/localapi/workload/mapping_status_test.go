package workload

import (
	"testing"
	"time"

	workloadapi "ardents/internal/workload"

	"github.com/stretchr/testify/require"
)

func TestWorkloadStatusMappingExposesCachedObservationMetadata(t *testing.T) {
	observedAt := time.Date(2032, 4, 5, 6, 7, 8, 0, time.UTC)
	snapshot, err := toWorkloadStatusSnapshot(workloadapi.StatusSnapshot{
		Spec:                workloadapi.SpecSnapshot{ID: "work.cached", Kind: "service"},
		Observed:            "degraded",
		ObservationDegraded: true,
		ObservedAt:          observedAt,
	})
	require.NoError(t, err)
	require.True(t, snapshot.GetObservationDegraded())
	require.Equal(t, observedAt, snapshot.GetObservedAt().AsTime())
}
