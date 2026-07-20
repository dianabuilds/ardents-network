package operation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNormalizeRepairsBrokenPersistedOperation(t *testing.T) {
	now := time.Date(2026, 3, 22, 18, 0, 0, 0, time.UTC)
	item, changed := Normalize(Record{
		Kind:      "node.startup.workloads",
		State:     "broken",
		Domain:    "workload",
		Resource:  "workloads",
		UpdatedAt: now,
	}, now)
	require.True(t, changed, "expected changed = true")
	require.Falsef(t, item.State != Recovering, "state = %q, want recovering", item.State)
	require.False(t, item.ID == "", "expected recovered id")
}

func TestPendingItemsReturnsStableTimeOrder(t *testing.T) {
	earlier := time.Date(2026, 3, 22, 18, 0, 0, 0, time.UTC)
	later := earlier.Add(time.Minute)
	items := PendingItems(map[string]Record{
		"b": {ID: "b", State: Running, StartedAt: later},
		"a": {ID: "a", State: Recovering, StartedAt: earlier},
		"c": {ID: "c", State: Completed, StartedAt: earlier},
	})
	require.Falsef(t, len(items) != 2, "len = %d, want 2", len(items))
	require.Falsef(t, items[0].ID != "a" || items[1].ID != "b", "order = %#v, want a then b", items)
}
