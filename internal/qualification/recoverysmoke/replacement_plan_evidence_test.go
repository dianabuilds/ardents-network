package recoverysmoke

import (
	"encoding/json"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestRoutePlanTimingUsesExactRuntimeAttachment(t *testing.T) {
	raw, err := json.Marshal(route.Evidence{Kind: "complete", Role: "rendezvous", Attachment: 2,
		DeadlineMillis: 2_000, LifetimeMillis: 60_000, Terminal: "error"})
	if err != nil {
		t.Fatal(err)
	}
	timing, err := parseRoutePlanTiming(append(raw, '\n'), "rendezvous", 2)
	if err != nil {
		t.Fatal(err)
	}
	if timing.Attachment != 2 || timing.DeadlineMillis != 2_000 || timing.LifetimeMillis != 60_000 {
		t.Fatalf("runtime timing differs: %+v", timing)
	}
}

func TestRoutePlanTimingRejectsMissingDuplicateAndMalformedEvidence(t *testing.T) {
	valid, err := json.Marshal(route.Evidence{Kind: "complete", Role: "client", Attachment: 1,
		DeadlineMillis: 2_000, LifetimeMillis: 60_000})
	if err != nil {
		t.Fatal(err)
	}
	for name, raw := range map[string][]byte{
		"missing":   nil,
		"duplicate": append(append(append([]byte(nil), valid...), '\n'), append(valid, '\n')...),
		"malformed": []byte("{not-json}\n"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseRoutePlanTiming(raw, "client", 1); err == nil {
				t.Fatal("invalid runtime Route timing was accepted")
			}
		})
	}
}

func TestReplacementCandidateUsesProcessLocalAttachmentOrdinal(t *testing.T) {
	first := processEvidenceRef{Identity: "first"}
	late := processEvidenceRef{Identity: "late"}
	proposals := []replacementProposal{{Attachment: 1}, {Attachment: 2}, {Attachment: 3}}
	proposals[0].Processes[0].Host = first
	proposals[1].Processes[0].Host = first
	proposals[2].Processes[0].Host = late
	if got := replacementLocalAttachment(proposals, 1, "initiator", first); got != 2 {
		t.Fatalf("reused process local Attachment = %d; want 2", got)
	}
	if got := replacementLocalAttachment(proposals, 2, "initiator", late); got != 1 {
		t.Fatalf("late process local Attachment = %d; want 1", got)
	}
}
