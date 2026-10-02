package hosting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

func TestSharedHostingObservationFreshness(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name      string
		next      time.Duration
		age       time.Duration
		end       time.Duration
		wantReads int
	}{
		{"pre-aged observation expires", 1800 * time.Millisecond, time.Second, time.Hour, 2},
		{"fresh observation reused", 950 * time.Millisecond, time.Second, time.Hour, 1},
		{"stricter caller refreshes", 950 * time.Millisecond, 100 * time.Millisecond, time.Hour, 2},
		{"period end refreshes", time.Second, time.Second, time.Second, 2},
		{"invalid age reaches owner", 950 * time.Millisecond, 2 * time.Second, time.Hour, 2},
		{"clock reversal reaches owner", -time.Millisecond, time.Second, time.Hour, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			now := base.Add(900 * time.Millisecond)
			reads := 0
			sampler := &sharedClosedHostingSampler{}
			read := func(_ context.Context, age time.Duration) (resource.HostingSample, error) {
				reads++
				if age > time.Second {
					return resource.HostingSample{}, errors.New("invalid freshness")
				}
				at := base
				if reads > 1 {
					at = now
				}
				return resource.HostingSample{At: at, Policy: resource.HostingPolicy{Start: base.Add(-time.Hour), End: base.Add(test.end)}, Observation: resource.HostingObservation{Drain: !now.Before(base.Add(test.end))}}, nil
			}
			if _, err := sampler.sample(t.Context(), time.Second, func() time.Time { return now }, read); err != nil {
				t.Fatal(err)
			}
			now = base.Add(test.next)
			result, err := sampler.sample(t.Context(), test.age, func() time.Time { return now }, read)
			if reads != test.wantReads {
				t.Fatalf("owner reads = %d, want %d; observation At=%v", reads, test.wantReads, result.At)
			}
			if test.age > time.Second && err == nil {
				t.Fatal("invalid freshness bypassed owner refusal")
			}
			if test.name == "period end refreshes" && !result.Observation.Drain {
				t.Fatal("expired period retained pre-expiry Drain=false")
			}
		})
	}
}

func TestSharedHostingJoinedFlightRechecksCaller(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name        string
		age         time.Duration
		invalidated bool
		end         time.Duration
		cancel      bool
		fail        bool
		wantReads   int
	}{
		{"fresh waiter coalesces", time.Second, false, time.Hour, false, false, 0},
		{"stricter waiter refreshes", 100 * time.Millisecond, false, time.Hour, false, false, 1},
		{"invalidated waiter refreshes", time.Second, true, time.Hour, false, false, 1},
		{"expired waiter refreshes", time.Second, false, 900 * time.Millisecond, false, false, 1},
		{"canceled waiter refuses", time.Second, false, time.Hour, true, false, 0},
		{"failed flight refuses", time.Second, false, time.Hour, false, true, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			sampler := &sharedClosedHostingSampler{}
			started, finish, joined := make(chan struct{}), make(chan struct{}), make(chan struct{})
			leaderDone := make(chan error, 1)
			failure := errors.New("measurement unavailable")
			clock := func() time.Time { return base.Add(900 * time.Millisecond) }
			go func() {
				_, err := sampler.sample(t.Context(), time.Second, clock, func(context.Context, time.Duration) (resource.HostingSample, error) {
					close(started)
					<-finish
					if test.fail {
						return resource.HostingSample{}, failure
					}
					return resource.HostingSample{At: base, Policy: resource.HostingPolicy{Start: base.Add(-time.Hour), End: base.Add(test.end)}}, nil
				})
				leaderDone <- err
			}()
			<-started
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			reads := 0
			waiterDone := make(chan error, 1)
			go func() {
				signaled := false
				_, err := sampler.sample(ctx, test.age, func() time.Time {
					if !signaled {
						close(joined)
						signaled = true
					}
					return clock()
				}, func(_ context.Context, age time.Duration) (resource.HostingSample, error) {
					reads++
					if age != test.age {
						return resource.HostingSample{}, errors.New("waiter freshness changed")
					}
					return resource.HostingSample{At: clock(), Policy: resource.HostingPolicy{Start: base.Add(-time.Hour), End: base.Add(time.Hour)}}, nil
				})
				waiterDone <- err
			}()
			<-joined
			// The waiter signals inside the sampler lock. Acquire it to prove it has
			// selected the active flight before completing the leader.
			sampler.mu.Lock()
			active := sampler.active
			sampler.mu.Unlock()
			if active == nil {
				t.Fatal("leader flight completed before waiter joined")
			}
			if test.invalidated {
				sampler.invalidate()
			}
			if test.cancel {
				cancel()
			}
			close(finish)
			leaderErr := <-leaderDone
			err := <-waiterDone
			if test.fail {
				if !errors.Is(leaderErr, failure) || !errors.Is(err, failure) {
					t.Fatalf("flight failure lost: leader=%v waiter=%v", leaderErr, err)
				}
			} else if leaderErr != nil {
				t.Fatal(leaderErr)
			}
			if test.cancel {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("canceled waiter returned %v", err)
				}
			} else if !test.fail && err != nil {
				t.Fatal(err)
			}
			if reads != test.wantReads {
				t.Fatalf("waiter owner reads = %d, want %d", reads, test.wantReads)
			}
		})
	}
}
