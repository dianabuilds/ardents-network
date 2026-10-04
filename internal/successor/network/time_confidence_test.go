package network_test

import (
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
)

func TestTimeConfidenceDoesNotReviveFactsAfterRollback(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	for _, test := range []struct {
		name                         string
		wall, monotonic, independent time.Time
		floor                        int64
		want                         time.Time
	}{
		{"agreement", now, now, now, now.Unix(), now},
		{"wall rollback with continuity", now.Add(-time.Hour), now, now, now.Unix(), now},
		{"tolerated correction holds floor", now, now, now, now.Unix() + 2, now.Add(2 * time.Second)},
		{"rollback behind floor", now, now, now, now.Unix() + 3, time.Time{}},
		{"missing observation", now, now, time.Time{}, 0, time.Time{}},
		{"stale observation", now, now, now.Add(-2*time.Second - time.Nanosecond), 0, time.Time{}},
		{"future observation", now, now, now.Add(2*time.Second + time.Nanosecond), 0, time.Time{}},
		{"wall jump forward", now.Add(time.Hour), now, now, 0, time.Time{}},
		{"exact tolerance", now, now, now.Add(-2 * time.Second), 0, now},
	} {
		t.Run(test.name, func(t *testing.T) {
			observation, err := network.ConfirmTime(network.ClockEvidence{Wall: test.wall, Monotonic: test.monotonic, Independent: test.independent}, test.floor)
			if test.want.IsZero() {
				if !errors.Is(err, network.ErrClockUncertain) || !observation.Instant().IsZero() {
					t.Fatalf("uncertain clock produced authority: %v", err)
				}
			} else if err != nil || !observation.Instant().Equal(test.want) {
				t.Fatalf("time=%v err=%v want=%v", observation.Instant(), err, test.want)
			}
		})
	}
	if !(network.TrustedTime{}).Instant().IsZero() {
		t.Fatal("zero observation is authoritative")
	}
}
