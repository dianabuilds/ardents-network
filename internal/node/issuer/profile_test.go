package issuer

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

func TestCurrentProfileRequiresCurrentStateProjection(t *testing.T) {
	snapshot := state.NodeDuty{Profile: carrier.ClosedRouteProfile, Fresh: true, NodeID: [32]byte{1},
		ValidUntil: time.Now().Add(time.Hour), RecordValidUntil: time.Now().Add(time.Hour)}
	for _, source := range []authority.Source{{}, {CurrentProfile: func() (state.ClosedProfileView, bool) {
		return state.ClosedProfileView{}, false
	}}} {
		if _, available := currentProfile(source, snapshot, time.Now()); available {
			t.Fatal("issuer accepted an unavailable current profile")
		}
	}
}
