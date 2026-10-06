package reachability_test

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
)

// These controls exercise local monotonic rules/lifetimes only. Supplied guards
// are failure probes, not authenticated Network, genuine Source or readiness.
func TestLookupHistoryOriginalVerificationAndHandoffGuards(t *testing.T) {
	for _, failure := range []int{3, 4} {
		t.Run(string(rune('0'+failure)), func(t *testing.T) {
			raw, target, network, profile, at := signedDescriptor(3)
			history := &reachability.History{}
			defer func() { _ = history.Close() }()
			lost := errors.New("original Source lost")
			checks := 0
			flight, err := history.Begin(context.Background(), target, network, profile, func() error {
				checks++
				if checks == failure {
					return lost
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			defer flight.Finish()
			if proof, err := flight.Complete(raw, at); !errors.Is(err, lost) || len(proof.Bytes()) != 0 {
				t.Fatal("lost original Source exported proof", err)
			}
			flight.Finish()
			older := resignDescriptor(raw, func(b []byte) { binary.BigEndian.PutUint64(b[162:170], 8) })
			next, err := history.Begin(context.Background(), target, network, profile, func() error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			_, err = next.Complete(older, at)
			next.Finish()
			if failure == 3 && err != nil {
				t.Fatal("pre-floor refusal mutated history", err)
			}
			if failure == 4 && err == nil {
				t.Fatal("post-floor refusal erased observed monotonic fact")
			}
		})
	}
}

func TestLookupHistoryConflictsRetainAcrossWorkerLoss(t *testing.T) {
	raw, target, network, profile, at := signedDescriptor(3)
	history := &reachability.History{}
	defer func() { _ = history.Close() }()
	accept := func(raw []byte, wantError bool) {
		t.Helper()
		flight, err := history.Begin(context.Background(), target, network, profile, func() error { return nil })
		if err != nil {
			t.Fatal(err)
		}
		_, err = flight.Complete(raw, at)
		flight.Finish()
		if (err != nil) != wantError {
			t.Fatal("unexpected history decision", err)
		}
	}
	accept(raw, false)
	accept(raw, false)
	conflict := resignDescriptor(raw, func(b []byte) { b[202] ^= 1 })
	accept(conflict, true)
	accept(raw, true)
	higher := resignDescriptor(raw, func(b []byte) { binary.BigEndian.PutUint64(b[162:170], 10) })
	accept(higher, false)
	// An interrupted worker flight never clears the context's retained facts.
	worker, cancel := context.WithCancel(context.Background())
	flight, err := history.Begin(worker, target, network, profile, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	defer flight.Finish()
	cancel()
	if _, err := flight.Complete(raw, at); !errors.Is(err, context.Canceled) {
		t.Fatal("dead worker accepted result", err)
	}
	flight.Finish()
	accept(raw, true)
	accept(higher, false)
}

func TestLookupHistoryCapacityBeforeEffectsAndOriginalFlightIdentity(t *testing.T) {
	raw, target, network, profile, at := signedDescriptor(3)
	history := &reachability.History{}
	defer func() { _ = history.Close() }()
	for i := 0; i < 128; i++ {
		proof, key, _, _, _ := signedDescriptorForAuthority(3, byte(0x11+i))
		flight, err := history.Begin(context.Background(), key, network, profile, func() error { return nil })
		if err != nil {
			t.Fatal(i, err)
		}
		if _, err := flight.Complete(proof, at); err != nil {
			flight.Finish()
			t.Fatal(i, err)
		}
		flight.Finish()
	}
	_, extra, _, _, _ := signedDescriptorForAuthority(3, 0xfe)
	effects := 0
	if flight, err := history.Begin(context.Background(), extra, network, profile, func() error { effects++; return nil }); err == nil {
		flight.Finish()
		t.Fatal("129th Target admitted")
	}
	if effects != 0 {
		t.Fatal("capacity refusal entered effects")
	}
	first, err := history.Begin(context.Background(), target, network, profile, func() error { return nil })
	if err != nil {
		t.Fatal("existing Target refused at capacity", err)
	}
	defer first.Finish()
	if other, err := history.Begin(context.Background(), target, network, profile, func() error { return nil }); err == nil {
		other.Finish()
		t.Fatal("original flight replaced")
	}
	first.Finish()
	second, err := history.Begin(context.Background(), target, network, profile, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	defer second.Finish()
	first.Finish()
	if _, err := first.Complete(raw, at); err == nil {
		t.Fatal("late original completed into replacement")
	}
	if _, err := second.Complete(raw, at); err != nil {
		t.Fatal("late Finish released replacement", err)
	}
	if _, err := second.Complete(raw, at); err == nil {
		t.Fatal("flight completed twice")
	}
}

func TestLookupHistoryRetirementInterruptsAndJoins(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, target, network, profile, _ := signedDescriptor(3)
		history := &reachability.History{}
		flight, err := history.Begin(context.Background(), target, network, profile, func() error { return nil })
		if err != nil {
			t.Fatal(err)
		}
		closed := make(chan struct{})
		go func() { _ = history.Close(); close(closed) }()
		synctest.Wait()
		select {
		case <-flight.Context().Done():
		default:
			t.Fatal("retirement did not interrupt flight")
		}
		select {
		case <-closed:
			t.Fatal("retirement cleared before physical join")
		default:
		}
		if other, err := history.Begin(context.Background(), target, network, profile, func() error { return nil }); err == nil {
			other.Finish()
			t.Fatal("retired context admitted flight")
		}
		flight.Finish()
		synctest.Wait()
		select {
		case <-closed:
		default:
			t.Fatal("retirement did not complete after join")
		}
		if err := history.Close(); err != nil {
			t.Fatal(err)
		}
	})
}

// A neighboring observer may block on actual durable I/O. Local retirement must
// still interrupt the original flight, then wait for its joined Finish.
func TestLookupHistoryRetirementDuringBlockedObserver(t *testing.T) {
	for _, phase := range []int{2, 3, 4} {
		t.Run(string(rune('0'+phase)), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				raw, target, network, profile, at := signedDescriptor(3)
				history := &reachability.History{}
				entered, release := make(chan struct{}), make(chan struct{})
				checks := 0
				flight, err := history.Begin(context.Background(), target, network, profile, func() error {
					checks++
					if checks == phase {
						close(entered)
						<-release
					}
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				completed := make(chan error, 1)
				go func() {
					proof, err := flight.Complete(raw, at)
					if len(proof.Bytes()) != 0 {
						err = errors.New("retired observer exported proof")
					}
					flight.Finish()
					completed <- err
				}()
				<-entered
				closed := make(chan error, 1)
				go func() { closed <- history.Close() }()
				synctest.Wait()
				if !errors.Is(flight.Context().Err(), context.Canceled) {
					t.Error("blocked observer prevented retirement interruption")
				}
				select {
				case <-closed:
					t.Error("context returned before original flight joined")
				default:
				}
				close(release)
				if err := <-completed; err == nil {
					t.Error("retired lookup accepted proof")
				}
				if err := <-closed; err != nil {
					t.Error(err)
				}
			})
		})
	}
}

func TestLookupHistoryIndependentReadBindings(t *testing.T) {
	raw, target, network, profile, at := signedDescriptor(2)
	for _, wrong := range []struct{ target, network, profile [32]byte }{
		{[32]byte{1}, network, profile}, {target, [32]byte{1}, profile}, {target, network, [32]byte{1}},
	} {
		history := &reachability.History{}
		flight, err := history.Begin(context.Background(), wrong.target, wrong.network, wrong.profile, func() error { return nil })
		if err != nil {
			t.Fatal(err)
		}
		if _, err := flight.Complete(raw, at); err == nil {
			t.Fatal("proof replaced independently selected lookup binding")
		}
		flight.Finish()
		if err := history.Close(); err != nil {
			t.Fatal(err)
		}
	}
	history := &reachability.History{}
	flight, err := history.Begin(context.Background(), target, network, profile, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := flight.Complete(raw, at); err != nil {
		t.Fatal("read incorrectly requires publication capability", err)
	}
	flight.Finish()
	if err := history.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLookupHistoryPublicationConflictCannotShortenExpiry(t *testing.T) {
	raw, target, network, profile, at := signedDescriptor(2)
	history := &reachability.History{}
	defer func() { _ = history.Close() }()
	accept := func(proof []byte, now time.Time, wantError bool) {
		t.Helper()
		flight, err := history.Begin(context.Background(), target, network, profile, func() error { return nil })
		if err != nil {
			t.Fatal(err)
		}
		defer flight.Finish()
		_, err = flight.Complete(proof, now)
		if (err != nil) != wantError {
			t.Fatal("unexpected publication floor decision", err)
		}
	}
	accept(raw, at, false)
	credential := 284 + len("ardents-service-publication-v3\x00")
	for _, hours := range []int{2, 3, 1} {
		conflict := resignDescriptor(raw, func(b []byte) {
			binary.BigEndian.PutUint64(b[credential+114:credential+122], uint64(at.Add(time.Duration(hours)*time.Hour).Unix()))
			b[credential+222] = byte(hours)
		})
		accept(conflict, at, true)
	}
	for _, hours := range []int{2, 3} {
		start := at.Add(time.Duration(hours) * time.Hour)
		successor := resignDescriptor(raw, func(b []byte) {
			binary.BigEndian.PutUint64(b[credential+98:credential+106], 8)
			binary.BigEndian.PutUint64(b[credential+106:credential+114], uint64(start.Unix()))
			binary.BigEndian.PutUint64(b[credential+114:credential+122], uint64(start.Add(time.Hour).Unix()))
			binary.BigEndian.PutUint64(b[266:274], uint64(start.Unix()))
			binary.BigEndian.PutUint64(b[274:282], uint64(start.Add(600*time.Second).Unix()))
		})
		accept(successor, start, hours == 2)
	}
}
