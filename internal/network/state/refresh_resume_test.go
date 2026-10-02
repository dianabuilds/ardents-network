package state

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/source"
)

func TestRefreshResumedDigestOutcomeSurvivesCompletionAndReopen(t *testing.T) {
	for _, test := range []struct {
		name            string
		status, outcome byte
	}{
		{"completed-unverified", sourceAttemptCompleted, 0},
		{"in-flight", sourceAttemptInFlight, 0},
		{"failed-recorded", sourceAttemptFailed, sourceOutcomeAuthentication},
		{"completed-recorded", sourceAttemptCompleted, sourceOutcomeValid},
	} {
		t.Run(test.name, func(t *testing.T) {
			initial, decision := newControlCommitFixture(t, false)
			now := time.Now().UTC().Truncate(time.Second)
			clock := func() time.Time { return now }
			initial.config.clock, initial.config.observe = clock, clock
			initial.config.anchorWall, initial.config.anchorMono = now, time.Now()
			initial.config.sourceInfo = source.Details{Configured: true, OrderSeed: sha256.Sum256([]byte("resume-order")),
				Identities: [2][32]byte{{1}, {2}}, Exposures: [2][32]byte{{3}, {4}}, Families: [2]string{"source-one", "source-two"}}
			order, deadline, err := initial.startSourceWave(now)
			if err != nil {
				t.Fatal(err)
			}
			if started, _, err := initial.beginLatestAttempt(order[0]); err != nil || !started {
				t.Fatalf("first latest: %t %v", started, err)
			}
			digestSlot := digestAttemptSlot(order[1])
			requested := [32]byte{9}
			if err := initial.beginDigestAttempt(order[1], requested); err != nil {
				t.Fatal(err)
			}
			if test.status == sourceAttemptCompleted {
				if err := initial.finishDigestAttempt(order[1], true); err != nil {
					t.Fatal(err)
				}
			}
			if test.outcome != 0 {
				recorded := initial.distribution
				recorded.sequence++
				recorded.attempts[digestSlot], recorded.outcomes[digestSlot] = test.status, test.outcome
				recorded.observedEpochs[digestSlot], recorded.observedDigests[digestSlot] = 7, requested
				if err := initial.commitDistribution(recorded); err != nil {
					t.Fatal(err)
				}
			}
			// Close the physical root at the interrupted publication boundary, without
			// executing a graceful wave completion that a crashed process could not run.
			if err := initial.storage.Close(); err != nil {
				t.Fatal(err)
			}
			storage, err := openTestDurableRoot(initial.config.root)
			if err != nil {
				t.Fatal(err)
			}
			defer storage.Close()
			resumed := &networkState{config: initial.config, current: &decision, storage: storage, workContext: context.Background()}
			if err := resumed.loadDistributionState(); err != nil {
				t.Fatal(err)
			}
			if err := resumed.recoverSourceWaveGuard(); err != nil {
				t.Fatal(err)
			}
			calls := 0
			resumed.config.fetchSourceOperation = func(ctx context.Context, index int, request source.Message) (source.Message, error) {
				calls++
				if index != order[1] || request.Operation != "latest" {
					t.Errorf("replayed consumed selector: source=%d operation=%s", index, request.Operation)
				}
				if got, ok := ctx.Deadline(); !ok || !got.Equal(deadline) {
					t.Errorf("deadline changed: %v %t", got, ok)
				}
				return source.Message{Status: "busy"}, nil
			}
			if _, err := resumed.Refresh(context.Background()); !errors.Is(err, errRefreshUnavailable) {
				t.Fatalf("Refresh=%v", err)
			}
			if calls != 1 {
				t.Fatalf("source operations=%d; want only unstarted LATEST", calls)
			}
			want := test.outcome
			if want == 0 {
				want = sourceOutcomeInterrupted
			}
			check := func(s *networkState) {
				t.Helper()
				d := s.distribution
				if d.outcomes[digestSlot] != want {
					t.Errorf("digest outcome=%s; want %s", sourceOutcomeName(d.outcomes[digestSlot]), sourceOutcomeName(want))
				}
				if d.outcomes[order[0]] != sourceOutcomeInterrupted || d.outcomes[order[1]] != sourceOutcomeBusy {
					t.Errorf("LATEST outcomes=%v", d.outcomes)
				}
				if d.cycleActive || d.sourceOrder != [2]byte{byte(order[0]), byte(order[1])} || d.cycleDeadline != deadline.Unix() {
					t.Errorf("cycle identity changed at completion: %+v", d)
				}
				if d.requestedDigests[order[1]] != requested || d.attempts[digestSlot] == sourceAttemptNotStarted {
					t.Error("consumed digest selector lost")
				}
				if test.outcome != 0 && (d.observedEpochs[digestSlot] != 7 || d.observedDigests[digestSlot] != requested) {
					t.Error("recorded evidence lost")
				}
				expected := d
				expected.consecutiveFailures = 0
				applyFailureBackoff(&expected, now)
				if d.consecutiveFailures != 1 || d.backoffLevel != expected.backoffLevel || d.nextAutomatic != expected.nextAutomatic {
					t.Error("failure backoff changed")
				}
				snapshot, err := s.Current()
				if err != nil || snapshot.SourceOutcomes[digestSlot] != sourceOutcomeName(want) {
					t.Errorf("diagnostic outcome=%v err=%v", snapshot.SourceOutcomes, err)
				}
			}
			check(resumed)
			if err := storage.Close(); err != nil {
				t.Fatal(err)
			}
			storage, err = openTestDurableRoot(initial.config.root)
			if err != nil {
				t.Fatal(err)
			}
			defer storage.Close()
			reopened := &networkState{config: resumed.config, current: &decision, storage: storage, workContext: context.Background()}
			if err := reopened.loadDistributionState(); err != nil {
				t.Fatal(err)
			}
			if err := reopened.recoverSourceWaveGuard(); err != nil {
				t.Fatal(err)
			}
			check(reopened)
			if _, err := reopened.Refresh(context.Background()); !errors.Is(err, errRefreshUnavailable) || calls != 1 {
				t.Fatalf("durable backoff allowed contact: %v calls=%d", err, calls)
			}
			now = time.Unix(reopened.distribution.nextAutomatic, 0)
			reopened.config.anchorWall, reopened.config.anchorMono = now, time.Now()
			if _, _, err := reopened.startSourceWave(now); err != nil {
				t.Fatal(err)
			}
			if reopened.distribution.outcomes != [4]byte{} || reopened.distribution.attempts != [4]byte{} || reopened.distribution.requestedDigests != [2][32]byte{} || reopened.distribution.observedEpochs != [4]uint64{} || reopened.distribution.observedDigests != [4][32]byte{} {
				t.Fatal("new cycle retained old observations or selectors")
			}
		})
	}
}
