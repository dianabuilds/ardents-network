package connection

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"testing"
)

func TestClosedNativeRecordRoundTrips(t *testing.T) {
	t.Parallel()
	context, err := Context(ContextInput{Network: [32]byte{1}, Target: [32]byte{2}, InstancePublic: [32]byte{3},
		PublicationDigest: [32]byte{4}, InstanceGeneration: 5, CandidateView: [32]byte{6},
		IsolationContext: [32]byte{7}, DestinationBinding: [32]byte{8}, WorkSafetyNotAfter: 10,
		WorkSafetyMaximum: 11, NoNewRecoveryAfter: 9})
	if err != nil {
		t.Fatal(err)
	}
	if got := "0ea52c142f054a51785356cdc539c694b42ce48b8b14d7770d9845485d9dfba7"; fmt.Sprintf("%x", context) != got {
		t.Fatalf("ConnectionContext = %x, want %s", context, got)
	}
	challenge := Challenge{Network: [32]byte{1}, Target: [32]byte{2}, InstanceGeneration: 5, Context: context, Nonce: [32]byte{9}}
	digest, err := ChallengeDigest(challenge)
	if err != nil {
		t.Fatal(err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var signature [64]byte
	copy(signature[:], ed25519.Sign(private, digest[:]))
	continuity, err := NewContinuity([32]byte{12}, RoleClient, 2, 3, 4, 5, context, [32]byte{13})
	if err != nil {
		t.Fatal(err)
	}
	records := []Record{{Challenge: &challenge}, {Proof: &Proof{ChallengeDigest: digest, Signature: signature}},
		{Continuity: &continuity}, {Data: &Data{AttachmentGeneration: 2, Offset: 3, Payload: []byte{1, 2}}},
		{Acknowledgement: &Acknowledgement{AttachmentGeneration: 2, Offset: 5}}, {Terminal: &Terminal{AttachmentGeneration: 2, Offset: 6}}}
	for _, record := range records {
		var wire bytes.Buffer
		if err := Write(&wire, record); err != nil {
			t.Fatalf("Write(%+v): %v", record, err)
		}
		parsed, err := Read(&wire)
		if err != nil {
			t.Fatalf("Read(%+v): %v", record, err)
		}
		switch {
		case record.Challenge != nil && parsed.Challenge == nil:
			t.Fatal("Challenge kind changed")
		case record.Proof != nil && (parsed.Proof == nil || !ed25519.Verify(public, parsed.Proof.ChallengeDigest[:], parsed.Proof.Signature[:])):
			t.Fatal("InstanceProof did not bind canonical Challenge digest")
		case record.Continuity != nil && parsed.Continuity == nil:
			t.Fatal("Continuity kind changed")
		case record.Data != nil && (parsed.Data == nil || !bytes.Equal(parsed.Data.Payload, record.Data.Payload)):
			t.Fatal("Data payload changed")
		case record.Acknowledgement != nil && parsed.Acknowledgement == nil:
			t.Fatal("Acknowledgement kind changed")
		case record.Terminal != nil && parsed.Terminal == nil:
			t.Fatal("Terminal kind changed")
		}
	}
	if err := VerifyContinuity([32]byte{12}, continuity, RoleClient, 2, context, [32]byte{13}); err != nil {
		t.Fatal(err)
	}
}

func TestNativeRecordRejectsProfileKindLengthAndContinuityMutations(t *testing.T) {
	t.Parallel()
	context := [32]byte{4}
	continuity, err := NewContinuity([32]byte{1}, RolePublisher, 1, 0, 1, 0, context, [32]byte{2})
	if err != nil {
		t.Fatal(err)
	}
	var wire bytes.Buffer
	if err := Write(&wire, Record{Continuity: &continuity}); err != nil {
		t.Fatal(err)
	}
	base := wire.Bytes()
	for _, mutation := range []struct {
		name  string
		at    int
		value byte
	}{
		{"prefix", 0, 'x'}, {"version", len(connectionPrefix) + 2, 2},
		{"profile", len(connectionPrefix) + 2 + 4, 'x'}, {"kind", len(connectionPrefix) + 2 + 2, 99},
	} {
		mutated := append([]byte(nil), base...)
		mutated[mutation.at] = mutation.value
		if _, err := Read(bytes.NewReader(mutated)); err == nil {
			t.Fatalf("%s mutation was accepted", mutation.name)
		}
	}
	continuity.MAC[0]++
	if err := VerifyContinuity([32]byte{1}, continuity, RolePublisher, 1, context, [32]byte{2}); err == nil {
		t.Fatal("Continuity MAC mutation was accepted")
	}
	if _, err := Read(bytes.NewReader(append(base[:len(base)-1], []byte{}...))); err == nil {
		t.Fatal("truncated record was accepted")
	}
}
