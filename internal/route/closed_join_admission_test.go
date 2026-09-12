package route

import (
	"testing"
	"time"
)

// Real admission/spend/outer lifetime owners; token verification and TLS
// exporter are explicit fixtures. This does not activate a JOIN data lane.
func TestClosedOuterDataJoinRequiresDataAdmissionAndRetainsOriginalLifetime(t *testing.T) {
	for _, class := range []uint8{1, 2, 3} {
		t.Run(string(rune('0'+class)), func(t *testing.T) {
			lane, bridge, lease, now := closedOuterAdmissionFixtureFor(t, ClosedPurposeDataJoin, class)
			original := lease.Deadline
			err := lane.admitVerified(lease)
			if class != 2 {
				if err == nil {
					t.Fatal("non-data admission activated DataJoin lifetime")
				}
				*now = now.Add(11 * time.Second)
				if _, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{2}}); err == nil {
					t.Fatal("wrong class outlived setup")
				}
				return
			}
			if err != nil {
				t.Fatalf("verified DataJoin admission refused: %v", err)
			}
			if err := lane.SetDeadline(original.Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
			if got := lane.lane.currentWriteDeadline(); got != original {
				t.Fatal("DataJoin extended original admission")
			}
			*now = now.Add(11 * time.Second)
			if _, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{2}}); err != nil {
				t.Fatalf("DataJoin carrier truncated to setup: %v", err)
			}
			if err := lane.admitVerified(lease); err == nil {
				t.Fatal("DataJoin admission reused")
			}
			*now = original
			if _, err := bridge.Accept(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{3}}); err == nil {
				t.Fatal("DataJoin outlived original admission")
			}
		})
	}
}
