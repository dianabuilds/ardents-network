package execution

import (
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/measurement"
	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"
)

func TestMeasuredCleanupSlotFollowsOriginalJoinAndRetainsFailure(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "joined", true: "failed"}[failed], func(t *testing.T) {
			capture, err := measurement.Start()
			if err != nil {
				t.Fatal(err)
			}
			defer capture.Stop()
			authority, _, session := executionFixture(t, Connection)
			job, err := session.BeginJob()
			if err != nil {
				t.Fatal(err)
			}
			var failure error
			if failed {
				failure = errors.New("private-cleanup-sentinel")
			}
			defer func() { job.Retire(); _ = job.Finish(failure) }()
			current := func() uint64 {
				t.Helper()
				capture.Sample()
				spec, _ := runtimeevent.ExecutionCleanupSlots.Spec()
				for _, v := range capture.Latest().Values {
					if v.Name == spec.Name {
						if !v.Available {
							t.Fatal("invalid measurement")
						}
						return v.Current
					}
				}
				t.Fatal("cleanup instrument absent")
				return 0
			}
			if err := authority.Revoke([32]byte{2}, Connection); err != nil {
				t.Fatal(err)
			}
			if authority.Active() != 0 || current() != 1 {
				t.Fatal("revoke returned physical cleanup capacity")
			}
			job.Retire()
			if got := job.Finish(failure); got != failure {
				t.Fatal(got)
			}
			if got := session.Close(); got != failure {
				t.Fatal(got)
			}
			want := uint64(0)
			if failed {
				want = 1
			}
			if got := current(); got != want {
				t.Fatalf("cleanup slot %d, want %d", got, want)
			}
		})
	}
}
