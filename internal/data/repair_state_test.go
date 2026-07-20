package data

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReplicaRepairBackoffIsDeterministicJitteredAndBounded(t *testing.T) {
	first := repairBackoff("repair-a", 1)
	require.GreaterOrEqual(t, first, 5*time.Second)
	require.LessOrEqual(t, first, 6*time.Second)
	require.Equal(t, first, repairBackoff("repair-a", 1))
	require.NotEqual(t, first, repairBackoff("repair-b", 1))

	previous := first
	for attempt := 2; attempt <= 12; attempt++ {
		current := repairBackoff("repair-a", attempt)
		require.GreaterOrEqual(t, current, previous)
		require.LessOrEqual(t, current, 5*time.Minute)
		previous = current
	}
}
