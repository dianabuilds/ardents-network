package state

import (
	"errors"
	"testing"
	"time"
)

// Each refusal is exercised against the same signed, persisted profile. The
// ordinary acceptance test supplies the genuine profile verifier and store;
// the observations here model owner failure, not replacement trust inputs.
func TestClosedProfileProjectionStopsWithStateOwner(t *testing.T) {
	for name, fail := range map[string]func(*networkState){
		"missing time owner":        func(s *networkState) { s.config.observe = nil },
		"automatic refresh failure": func(s *networkState) { s.automaticErr = errors.New("refresh failed") },
		"resource failure":          func(s *networkState) { s.resourceErr = errors.New("resource failed") },
		"missing time observation":  func(s *networkState) { s.config.observe = func() time.Time { return time.Time{} } },
		"stale time observation": func(s *networkState) {
			s.config.observe = func() time.Time { return s.config.clock().Add(-3 * time.Second) }
		},
		"rollback behind retained floor": func(s *networkState) { s.distribution.trustedTimeFloor = s.config.clock().Add(3 * time.Second).Unix() },
	} {
		t.Run(name, func(t *testing.T) {
			store, raw := closedProfileStoreFixture(t)
			if _, err := store.AcceptClosedProfile(raw); err != nil {
				t.Fatal(err)
			}
			if _, err := store.CurrentClosedProfile(); err != nil {
				t.Fatal(err)
			}
			if _, err := store.CurrentClosedRoute(); err != nil {
				t.Fatal(err)
			}
			fail(store)
			if value, err := store.CurrentClosedProfile(); err == nil || value != (ClosedProfileView{}) {
				t.Error("failed State owner exposed admission profile")
			}
			if value, err := store.CurrentClosedRoute(); err == nil || value != (ClosedRouteView{}) {
				t.Error("failed State owner exposed route recipients")
			}
		})
	}
}
