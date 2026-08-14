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

func TestRendezvousProposalSeparatesRouteTerminalFromConnectionCommit(t *testing.T) {
	positions := make([]route.Position, len(replacementRoles))
	var raw []byte
	for attachment := uint32(1); attachment <= 3; attachment++ {
		value := route.Evidence{Kind: "complete", Role: "client", Attachment: attachment,
			Terminal: "success", Positions: positions}
		if attachment == 3 {
			value.IntroductionSetupReceipt = [32]byte{1}
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		raw = append(raw, encoded...)
		raw = append(raw, '\n')
	}
	proposals, err := replacementProposals(raw, "isolated-rendezvous")
	if err != nil {
		t.Fatal(err)
	}
	if proposals[1].Terminal != "success" || proposals[1].Committed {
		t.Fatalf("failed Carrier proposal terminal=%s committed=%v", proposals[1].Terminal, proposals[1].Committed)
	}
}
