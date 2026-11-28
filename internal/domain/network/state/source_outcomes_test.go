package state

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	source2 "github.com/dianabuilds/ardents-network/internal/domain/network/source"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state/journal"
)

func TestMalformedBundleLengthPersistsFramingOutcome(t *testing.T) {
	s, current := newControlCommitFixture(t, false)
	_, decodeErr := source2.DecodeBundle([]byte("ARDH3B1\x00\x00\x00\x00\x00"))
	if decodeErr == nil {
		t.Fatal("malformed member length decoded")
	}
	result := failedSourceResult(0, 0, [4]byte{}, decodeErr)
	if _, err := s.completeSourceWave(time.Unix(1_800_000_100, 0), &current, []sourceResult{result}); !errors.Is(err, errRefreshUnavailable) {
		t.Fatalf("failed wave: %v", err)
	}
	_, raw, err := s.storage.LoadControl()
	if err != nil {
		t.Fatal(err)
	}
	stored, err := journal.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Outcomes[0] != journal.OutcomeFraming {
		t.Fatalf("persisted malformed bundle outcome=%s; want framing-failed", sourceOutcomeName(stored.Outcomes[0]))
	}
	snapshot, err := s.Current()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.SourceOutcomes[0] != "framing-failed" {
		t.Fatalf("snapshot malformed bundle outcome=%q; want framing-failed", snapshot.SourceOutcomes[0])
	}
}

func TestByDigestResponseRejectsDifferentObject(t *testing.T) {
	t.Parallel()
	requested, returned := [32]byte{1}, [32]byte{2}
	err := validateByDigestResponse(requested, returned)
	if got := classifySourceOutcome(err); got != journal.OutcomeFraming {
		t.Fatalf("BY_DIGEST object mismatch outcome=%s; want framing-failed", sourceOutcomeName(got))
	}
}

func TestProtocolStatusesKeepDistinctTerminalOutcomes(t *testing.T) {
	t.Parallel()
	want := [...]byte{journal.OutcomeNotFound, journal.OutcomeBusy, journal.OutcomeBadRequest, journal.OutcomeInternal}
	for status := byte(1); status < byte(len(sourceStatusErrors)); status++ {
		if got := classifySourceOutcome(sourceStatusErrors[status]); got != want[status-1] {
			t.Fatalf("status %d outcome=%d want=%d", status, got, want[status-1])
		}
	}
}

func TestUnknownSourceStatusIsRefused(t *testing.T) {
	t.Parallel()
	for status, want := range map[string]error{
		"not-found":   sourceStatusErrors[1],
		"busy":        sourceStatusErrors[2],
		"bad-request": sourceStatusErrors[3],
		"internal":    sourceStatusErrors[4],
	} {
		if got := sourceStatusError(status); !errors.Is(got, want) {
			t.Fatalf("status %q error=%v want=%v", status, got, want)
		}
	}
	// An unrecognized non-ok status must refuse, not return a nil success.
	err := sourceStatusError("unexpected-status")
	if err == nil {
		t.Fatal("unknown source status returned nil (fail-open)")
	}
	if got := classifySourceOutcome(err); got != journal.OutcomeInvalidState {
		t.Fatalf("unknown status outcome=%s want=invalid-state", sourceOutcomeName(got))
	}
}

func TestSourceOutcomeUsesCauseNotDiagnosticText(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want byte
	}{
		{"unavailable with framing words", fmt.Errorf("bundle length invalid: %w", source2.ErrUnavailable), journal.OutcomeUnavailable},
		{"authentication with resource words", fmt.Errorf("bound exceeded: %w", source2.ErrAuthentication), journal.OutcomeAuthentication},
		{"framing with authentication words", fmt.Errorf("certificate failed: %w", source2.ErrFraming), journal.OutcomeFraming},
		{"unknown words do not invent resource failure", errors.New("bundle length exceeded"), journal.OutcomeInvalidState},
		{"canceled before transport cause", errors.Join(context.Canceled, source2.ErrUnavailable), journal.OutcomeCanceled},
		{"deadline before transport cause", errors.Join(context.DeadlineExceeded, source2.ErrUnavailable), journal.OutcomeTimeout},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := classifySourceOutcome(test.err); got != test.want {
				t.Fatalf("outcome=%s want=%s for %v", sourceOutcomeName(got), sourceOutcomeName(test.want), test.err)
			}
		})
	}
}

func TestLegacyResourceOutcomeRemainsReadable(t *testing.T) {
	s, _ := newControlCommitFixture(t, false)
	next := s.distribution
	next.Sequence++
	next.Attempts[0] = journal.AttemptFailed
	next.Outcomes[0] = journal.OutcomeResource
	if err := s.commitDistribution(next); err != nil {
		t.Fatal(err)
	}
	_, raw, err := s.storage.LoadControl()
	if err != nil {
		t.Fatal(err)
	}
	stored, err := journal.Decode(raw)
	if err != nil {
		t.Fatalf("legacy resource outcome was not readable: %v", err)
	}
	if stored.Outcomes[0] != journal.OutcomeResource {
		t.Fatalf("legacy resource outcome was not readable: outcome=%d", stored.Outcomes[0])
	}
	snapshot, err := s.Current()
	if err != nil {
		t.Fatalf("legacy resource outcome snapshot: %v", err)
	}
	if snapshot.SourceOutcomes[0] != "resource-failed" {
		t.Fatalf("legacy resource outcome snapshot=%q", snapshot.SourceOutcomes[0])
	}
}

func TestSourceOutcomeJournalByteAssignments(t *testing.T) {
	// ARDS1D4 stores these values, including the retained resource category.
	got := [...]byte{
		journal.OutcomeValid, journal.OutcomeUnavailable, journal.OutcomeAuthentication,
		journal.OutcomeTimeout, journal.OutcomeFraming, journal.OutcomeResource,
		journal.OutcomeCanceled, journal.OutcomeInvalidState, journal.OutcomeInterrupted,
		journal.OutcomeNotFound, journal.OutcomeBusy, journal.OutcomeBadRequest,
		journal.OutcomeInternal,
	}
	want := [...]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13}
	if got != want {
		t.Fatalf("ARDS1D4 outcome bytes=%v want=%v", got, want)
	}
}
