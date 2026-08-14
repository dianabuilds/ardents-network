package routesmoke

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestAlignedRecoverySelectionReturnsSelectorErrorImmediately(t *testing.T) {
	wantErr := errors.New("selection failed")
	calls := 0
	selector := func(state.Snapshot, route.Selection) (route.Plan, error) {
		calls++
		return route.Plan{}, wantErr
	}
	_, _, err := alignedRecoverySelectionWith(state.Snapshot{}, [32]byte{1}, [32]byte{2}, time.Unix(1, 0), selector)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v; want selector error", err)
	}
	if calls != 1 {
		t.Fatalf("selector calls = %d; want 1", calls)
	}
	if !strings.Contains(err.Error(), "generation 0 during alignment attempt 0") {
		t.Fatalf("error lacks selection context: %v", err)
	}
}
