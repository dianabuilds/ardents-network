package route

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPreviewUnavailableDoesNotMutateRouteState(t *testing.T) {
	state := NewState()

	preview := state.PreviewUnavailable("transport is not active")
	require.Falsef(t, preview.Outcome !=
		"not_found", "preview outcome = %q, want not_found", preview.Outcome)
	require.Falsef(t, state.State() !=
		"new", "state = %q, want new", state.State())
	require.Falsef(t, state.Last() !=
		(Snapshot{}), "last = %#v, want zero snapshot", state.Last())

}
