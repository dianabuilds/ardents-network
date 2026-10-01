package route

import (
	"testing"
	"time"
)

func TestClosedBootstrapRefillTimePartitions(t *testing.T) {
	for _, step := range []time.Duration{time.Second, time.Millisecond, 10 * time.Microsecond} {
		for _, consume := range []bool{false, true} {
			t.Run(step.String()+"/consume="+map[bool]string{false: "false", true: "true"}[consume], func(t *testing.T) {
				now := time.Unix(1_800_000_000, 0).UTC()
				controller, err := NewClosedBootstrapController(func() time.Time { return now })
				if err != nil {
					t.Fatal(err)
				}
				admit := func() *ClosedBootstrapLease {
					t.Helper()
					lease, err := controller.Admit([32]byte{1}, now.Add(time.Minute))
					if err != nil {
						t.Fatal(err)
					}
					return lease
				}
				drain := admit()
				if err := drain.Send(closedBootstrapOutputBurst); err != nil {
					t.Fatal(err)
				}
				drain.Release()
				lane := admit()
				defer lane.Release()
				var sent uint64
				for elapsed := time.Duration(0); elapsed < time.Second; elapsed += step {
					now = now.Add(step)
					// A request above the remaining lane budget must not discard refill.
					if err := lane.Send(closedBootstrapLaneBytes + 1); err == nil {
						t.Fatal("oversized send accepted")
					}
					if consume {
						for lane.Send(1) == nil {
							sent++
						}
					}
				}
				expected := uint64(closedBootstrapOutputRate / 60)
				if !consume {
					if err := lane.Send(expected); err != nil {
						t.Fatalf("partitioned refill: %v", err)
					}
					sent = expected
				}
				if sent != expected {
					t.Fatalf("sent %d, want %d", sent, expected)
				}
				if err := lane.Send(1); err == nil {
					t.Fatal("exceeded one-second refill")
				}
				// Remaining fractional credit must carry into the next interval too.
				now = now.Add(time.Second)
				next := uint64(2*closedBootstrapOutputRate/60) - expected
				if err := lane.Send(next); err != nil {
					t.Fatalf("second refill: %v", err)
				}
				if err := lane.Send(1); err == nil {
					t.Fatal("exceeded two-second refill")
				}
			})
		}
	}
}

func TestClosedBootstrapRefillDiscardsCreditAtBurstCap(t *testing.T) {
	for _, idle := range []time.Duration{8*time.Second + time.Nanosecond, time.Hour} {
		t.Run(idle.String(), func(t *testing.T) {
			now := time.Unix(1_800_000_000, 0).UTC()
			controller, err := NewClosedBootstrapController(func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}
			seed, err := controller.Admit([32]byte{1}, now.Add(time.Minute))
			if err != nil {
				t.Fatal(err)
			}
			if err := seed.Send(1); err != nil {
				t.Fatal(err)
			}
			now = now.Add(57 * time.Microsecond)
			if err := seed.Send(closedBootstrapLaneBytes + 1); err == nil {
				t.Fatal("oversized send accepted")
			}
			seed.Release()
			now = now.Add(idle)
			lane, err := controller.Admit([32]byte{1}, now.Add(time.Minute))
			if err != nil {
				t.Fatal(err)
			}
			if err := lane.Send(closedBootstrapOutputBurst); err != nil {
				t.Fatal(err)
			}
			lane.Release()
			lane, err = controller.Admit([32]byte{1}, now.Add(time.Minute))
			if err != nil {
				t.Fatal(err)
			}
			defer lane.Release()
			now = now.Add(57 * time.Microsecond)
			if err := lane.Send(1); err == nil {
				t.Fatal("idle credit survived burst cap")
			}
			now = now.Add(time.Microsecond)
			if err := lane.Send(1); err != nil {
				t.Fatalf("fresh refill: %v", err)
			}
			if err := lane.Send(1); err == nil {
				t.Fatal("exceeded fresh refill")
			}
		})
	}
}
