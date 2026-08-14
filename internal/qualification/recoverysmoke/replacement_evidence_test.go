package recoverysmoke

import (
	"encoding/json"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestReplacementEvidenceParsersRejectMalformedRecords(t *testing.T) {
	malformed := []byte("{not-json}\n")
	if _, err := replacementProposals(malformed, "isolated-initiator"); err == nil {
		t.Fatal("replacement proposal parser skipped malformed JSON")
	}
	if _, _, _, _, err := introductionSetupEvidence(malformed, nil, nil); err == nil {
		t.Fatal("Introduction setup parser skipped malformed JSON")
	}
	if _, err := parseAttachmentCount(malformed, "introduction"); err == nil {
		t.Fatal("attachment parser skipped malformed JSON")
	}
}

func TestReplacementEvidenceParsersRejectDuplicateRelevantRecords(t *testing.T) {
	attachment := route.Evidence{Kind: "complete", Role: "introduction", Attachment: 1,
		Terminal: "success", PeerAuthenticated: true}
	raw, err := json.Marshal(attachment)
	if err != nil {
		t.Fatal(err)
	}
	duplicated := append(append(append([]byte(nil), raw...), '\n'), raw...)
	duplicated = append(duplicated, '\n')
	if _, err := parseAttachmentCount(duplicated, "introduction"); err == nil {
		t.Fatal("duplicate attachment evidence was accepted")
	}
	attachment.IntroductionSetupReceipt = [32]byte{1}
	raw, err = json.Marshal(attachment)
	if err != nil {
		t.Fatal(err)
	}
	duplicated = append(append(append([]byte(nil), raw...), '\n'), raw...)
	duplicated = append(duplicated, '\n')
	if _, _, _, _, err := introductionSetupEvidence(duplicated, duplicated, duplicated); err == nil {
		t.Fatal("duplicate Introduction setup evidence was accepted")
	}
}

func TestAttachmentEvidenceAcceptsStructuredTerminalError(t *testing.T) {
	attachment := route.Evidence{Kind: "complete", Role: "introduction", Attachment: 1,
		Terminal: "success", PeerAuthenticated: true}
	complete, err := json.Marshal(attachment)
	if err != nil {
		t.Fatal(err)
	}
	terminal, err := json.Marshal(map[string]string{
		"kind": "error", "error": "authenticate initiator: expected failure",
	})
	if err != nil {
		t.Fatal(err)
	}
	raw := append(append(append(complete, '\n'), terminal...), '\n')
	count, err := parseAttachmentCount(raw, "introduction")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("attachment count = %d; want 1", count)
	}
}
