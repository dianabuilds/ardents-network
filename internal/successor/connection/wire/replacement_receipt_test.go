package wire

import (
	"bytes"
	"crypto/tls"
	"testing"
)

// These private TLSBinding values isolate transcript/one-use ownership. They
// do not attest an actual TLS handshake, Instance lease, Job or Route JOIN.
func TestReplacementRetainsOriginalKeyAndFreshExporter(t *testing.T) {
	t.Parallel()
	original, fresh := [32]byte{1}, [32]byte{2}
	logical, exporter := [32]byte{3}, [32]byte{4}
	binding := &TLSBinding{connection: &tls.Conn{}, context: [32]byte{5}, key: fresh, exporter: exporter}
	peer, err := NewContinuity(original, RoleInitiator, 2, 7, 11, 13, logical, exporter)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name       string
		key        [32]byte
		value      Continuity
		role       Role
		generation uint64
		logical    [32]byte
	}{
		{"fresh-key", fresh, peer, RoleInitiator, 2, logical},
		{"wrong-role", original, peer, RolePublisher, 2, logical},
		{"initial-generation", original, peer, RoleInitiator, 1, logical},
		{"future-generation", original, peer, RoleInitiator, 3, logical},
		{"other-logical", original, peer, RoleInitiator, 2, [32]byte{6}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := VerifyReplacement(binding, test.key, test.value, test.role, test.generation, test.logical); err == nil {
				t.Fatal("foreign replacement accepted")
			}
		})
	}
	freshKeyPeer, _ := NewContinuity(fresh, RoleInitiator, 2, 7, 11, 13, logical, exporter)
	if _, err := VerifyReplacement(binding, original, freshKeyPeer, RoleInitiator, 2, logical); err == nil {
		t.Fatal("fresh TLS key took over the original logical Connection")
	}
	oldExporter, _ := NewContinuity(original, RoleInitiator, 2, 7, 11, 13, logical, [32]byte{7})
	if _, err := VerifyReplacement(binding, original, oldExporter, RoleInitiator, 2, logical); err == nil {
		t.Fatal("old Attachment exporter accepted")
	}
	altered := peer
	altered.ReceiveNext++
	if _, err := VerifyReplacement(binding, original, altered, RoleInitiator, 2, logical); err == nil {
		t.Fatal("altered offsets authenticated")
	}
	receipt, err := VerifyReplacement(binding, original, peer, RoleInitiator, 2, logical)
	if err != nil {
		t.Fatal(err)
	}
	commitment := ContinuityCommitment(original)
	copied := &ReplacementReceipt{self: receipt, binding: binding, peer: peer, keyCommitment: commitment}
	if _, err := copied.Consume(binding, RoleInitiator, 2, logical, commitment); err == nil {
		t.Fatal("copied receipt accepted")
	}
	foreign := &TLSBinding{connection: binding.connection, context: binding.context, key: fresh, exporter: exporter}
	if _, err := receipt.Consume(foreign, RoleInitiator, 2, logical, commitment); err == nil {
		t.Fatal("foreign binding consumed receipt")
	}
	if _, err := receipt.Consume(binding, RoleInitiator, 2, logical, ContinuityCommitment(fresh)); err == nil {
		t.Fatal("fresh key commitment consumed original receipt")
	}
	got, err := receipt.Consume(binding, RoleInitiator, 2, logical, commitment)
	if err != nil || got != peer {
		t.Fatal("refusal changed authenticated facts or consumed original", err)
	}
	if _, err := receipt.Consume(binding, RoleInitiator, 2, logical, commitment); err == nil {
		t.Fatal("receipt reused")
	}
	local, _ := NewContinuity(original, RolePublisher, 2, 9, 13, 11, logical, exporter)
	if _, err := NewReplacementResponse(receipt, fresh, local); err == nil {
		t.Fatal("fresh key produced response")
	}
	response, err := NewReplacementResponse(receipt, original, local)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewReplacementResponse(receipt, original, local); err == nil {
		t.Fatal("request issued another response")
	}
	copiedResponse := &ReplacementResponse{self: response, binding: binding, local: local, keyCommitment: commitment, body: response.body}
	if _, err := copiedResponse.Consume(binding, RolePublisher, 2, logical, commitment); err == nil {
		t.Fatal("copied response accepted")
	}
	if _, err := response.Consume(foreign, RolePublisher, 2, logical, commitment); err == nil {
		t.Fatal("response transferred to foreign TLS")
	}
	body, err := response.Consume(binding, RolePublisher, 2, logical, commitment)
	if err != nil {
		t.Fatal(err)
	}
	reader := bytes.NewReader(body)
	record, err := Read(reader)
	if err != nil || record.Continuity == nil || *record.Continuity != local || reader.Len() != 0 {
		t.Fatal("response was not one exact canonical Continuity flight", err)
	}
	if _, err := response.Consume(binding, RolePublisher, 2, logical, commitment); err == nil {
		t.Fatal("response reused")
	}
}

func TestReplacementRetirementDeniesPendingReceiptAndResponse(t *testing.T) {
	t.Parallel()
	for _, consume := range []bool{false, true} {
		binding := &TLSBinding{connection: &tls.Conn{}, context: [32]byte{1}, key: [32]byte{2}, exporter: [32]byte{3}}
		key, logical := [32]byte{4}, [32]byte{5}
		peer, _ := NewContinuity(key, RoleInitiator, 2, 0, 0, 0, logical, binding.exporter)
		receipt, err := VerifyReplacement(binding, key, peer, RoleInitiator, 2, logical)
		if err != nil {
			t.Fatal(err)
		}
		var response *ReplacementResponse
		if consume {
			if _, err := receipt.Consume(binding, RoleInitiator, 2, logical, ContinuityCommitment(key)); err != nil {
				t.Fatal(err)
			}
			local, _ := NewContinuity(key, RolePublisher, 2, 0, 0, 0, logical, binding.exporter)
			response, err = NewReplacementResponse(receipt, key, local)
			if err != nil {
				t.Fatal(err)
			}
		}
		binding.Erase()
		if _, err := VerifyReplacement(binding, key, peer, RoleInitiator, 2, logical); err == nil {
			t.Fatal("retired TLS issued receipt")
		}
		if !consume {
			if _, err := receipt.Consume(binding, RoleInitiator, 2, logical, ContinuityCommitment(key)); err == nil {
				t.Fatal("retired TLS consumed receipt")
			}
		} else if _, err := response.Consume(binding, RolePublisher, 2, logical, ContinuityCommitment(key)); err == nil {
			t.Fatal("retired TLS consumed response")
		}
	}
}
