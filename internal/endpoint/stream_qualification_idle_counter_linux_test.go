//go:build linux

package endpoint

import (
	"context"
	"errors"
	"testing"
	"time"

	hostingbudget "github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/resource"
)

func TestIdleCounterIntervalUsesMonotonicBoundaries(t *testing.T) {
	for _, wallElapsed := range []time.Duration{600 * time.Second, 660 * time.Second} {
		t.Run(wallElapsed.String(), func(t *testing.T) {
			base := time.Now()
			monotonic := base
			wall := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
			reads, observations := 0, 0
			var report StreamQualificationIdleReport
			err := observeStreamQualificationIdle(t.Context(), time.Millisecond, &report,
				func(ctx context.Context, fresh bool) (hostingbudget.Sample, resource.Sample, error) {
					if !fresh {
						t.Fatal("counter boundaries must be fresh")
					}
					reads++
					at := wall
					if reads == 1 {
						monotonic = base.Add(2 * time.Second)
					} else {
						at = wall.Add(wallElapsed)
						monotonic = monotonic.Add(4 * time.Second)
					}
					return hostingbudget.Sample{At: at}, resource.Sample{}, nil
				},
				func(ctx context.Context, event StreamQualificationEvent) error {
					observations++
					if observations == 1 {
						monotonic = base.Add(602 * time.Second)
					} else {
						monotonic = monotonic.Add(time.Hour)
					}
					return nil
				}, func() time.Time { return monotonic })
			if err != nil || report.MeasuredDuration != 600*time.Second || report.Samples != 2 {
				t.Fatalf("counter interval=%+v error=%v", report, err)
			}
			if !report.Started.Equal(wall) || !report.Stopped.Equal(wall.Add(wallElapsed)) {
				t.Fatalf("wall correlation lost: %+v", report)
			}
		})
	}
}

func TestIdleCounterObservationRetainsIncompleteOutcomes(t *testing.T) {
	failure := errors.New("counter continuity unavailable")
	for _, scenario := range []string{"counter", "callback", "drain", "cancel", "interval"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			now := time.Now()
			reads := 0
			var report StreamQualificationIdleReport
			err := observeStreamQualificationIdle(ctx, time.Millisecond, &report,
				func(context.Context, bool) (hostingbudget.Sample, resource.Sample, error) {
					reads++
					if reads == 2 && scenario == "counter" {
						return hostingbudget.Sample{}, resource.Sample{}, failure
					}
					host := hostingbudget.Sample{At: now.UTC()}
					host.Observation.Drain = scenario == "drain"
					return host, resource.Sample{}, nil
				},
				func(context.Context, StreamQualificationEvent) error {
					if scenario == "callback" {
						return failure
					}
					if scenario == "cancel" {
						cancel()
					}
					return nil
				}, func() time.Time { return now })
			if err == nil || report.MeasuredDuration != 0 {
				t.Fatalf("incomplete observation accepted: %+v / %v", report, err)
			}
			if scenario == "counter" || scenario == "callback" {
				if !errors.Is(err, failure) {
					t.Fatalf("original failure lost: %v", err)
				}
			}
			if scenario == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation lost: %v", err)
			}
		})
	}
}
