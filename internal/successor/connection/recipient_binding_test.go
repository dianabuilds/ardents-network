package connection

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"testing"
	"time"

	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/publication"
	"github.com/dianabuilds/ardents-network/internal/successor/route/capsule"
)

// Independently encoded public inputs grant no accepting Publisher, original
// worker or Service stream. Positive derivation tests exercise transcript rules.
func bindingInputs(t *testing.T) (publication.Proof, capsule.Request, time.Time) {
	t.Helper()
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x11}, 32))
	instance := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x22}, 32))
	pub := authority.Public().(ed25519.PublicKey)
	target := sha256.Sum256(append([]byte("ardents-service-target-v3\x00"), pub...))
	network := [32]byte{0x33}
	at := time.Unix(1900000000, 0).UTC()
	credential := []byte{0, 3}
	credential = append(credential, pub...)
	credential = append(credential, target[:]...)
	credential = append(credential, instance.Public().(ed25519.PublicKey)...)
	for _, number := range []uint64{7, uint64(at.Unix()), uint64(at.Add(time.Hour).Unix())} {
		credential = binary.BigEndian.AppendUint64(credential, number)
	}
	credential = append(credential, network[:]...)
	credential = binary.BigEndian.AppendUint32(credential, 3)
	credential = append(credential, ed25519.Sign(authority, credential)...)
	record := append([]byte("ardents-service-publication-v3\x00"), credential...)
	record = append(record, bytes.Repeat([]byte{0x44}, 32)...)
	commitment := sha256.Sum256(record)
	record = append(record, ed25519.Sign(instance, commitment[:])...)
	proof, err := publication.VerifyPublish(record, target, network, at)
	if err != nil {
		t.Fatal(err)
	}
	request := capsule.Request{Network: network, Target: target, PublicationDigest: proof.Digest(), Revision: 1,
		RendezvousNode: [32]byte{1}, RendezvousDutyGeneration: 2, JoinSecret: [32]byte{3}, HandshakeContext: [32]byte{4},
		ProfileDigest: [32]byte{5}, ConnectionNonce: [32]byte{6}, InitiatorBinding: [32]byte{7}, AttachmentGeneration: 1,
		Deadline: at.Add(10 * time.Second), WorkSafetyNotAfter: at.Add(300 * time.Second).Unix(),
		WorkSafetyMaximum: at.Add(600 * time.Second).Unix(), NoNewRecoveryAfter: at.Add(200 * time.Second).Unix()}
	return proof, request, at
}

func TestRecipientBindingTranscriptSeparatesLogicalAndAttachmentFacts(t *testing.T) {
	proof, request, now := bindingInputs(t)
	digest, err := request.Digest()
	if err != nil {
		t.Fatal(err)
	}
	binding, err := deriveRecipientBinding(proof, request.ProfileDigest, request, digest, now.Add(600*time.Second), now)
	if err != nil {
		t.Fatal(err)
	}
	// Explicit independent fixed-offset transcript, rather than calling a
	// production encoder. Offsets pin the u64 generation and safety ordering.
	raw := make([]byte, 283)
	copy(raw, []byte("ardents-service-context-v3\x00"))
	value := proof.Delegation()
	copy(raw[27:59], value.Network[:])
	copy(raw[59:91], value.Target[:])
	copy(raw[91:123], value.Instance[:])
	binary.BigEndian.PutUint64(raw[123:131], 7)
	publicationDigest := proof.Digest()
	copy(raw[131:163], publicationDigest[:])
	copy(raw[163:195], request.ProfileDigest[:])
	copy(raw[195:227], request.ConnectionNonce[:])
	copy(raw[227:259], request.InitiatorBinding[:])
	// The complete logical transcript is 283 bytes: 27-byte domain, seven
	// 32-byte fields, one generation and three bounds.
	binary.BigEndian.PutUint64(raw[259:267], uint64(request.WorkSafetyNotAfter))
	binary.BigEndian.PutUint64(raw[267:275], uint64(request.WorkSafetyMaximum))
	binary.BigEndian.PutUint64(raw[275:283], uint64(request.NoNewRecoveryAfter))
	logical, attachment := binding.Contexts()
	if logical != sha256.Sum256(raw) {
		t.Fatal("immutable logical transcript bytes differ")
	}
	attachmentRaw := make([]byte, 102)
	copy(attachmentRaw, []byte("ardents-attachment-context-v3\x00"))
	copy(attachmentRaw[30:62], logical[:])
	copy(attachmentRaw[62:94], digest[:])
	binary.BigEndian.PutUint64(attachmentRaw[94:102], 1)
	if attachment != sha256.Sum256(attachmentRaw) {
		t.Fatal("Attachment transcript bytes differ")
	}
	request.JoinSecret[0]++
	request.HandshakeContext[0]++
	changedDigest, err := request.Digest()
	if err != nil {
		t.Fatal(err)
	}
	changed, err := deriveRecipientBinding(proof, request.ProfileDigest, request, changedDigest, now.Add(600*time.Second), now)
	if err != nil {
		t.Fatal(err)
	}
	changedLogical, changedAttachment := changed.Contexts()
	if changedLogical != logical || changedAttachment == attachment {
		t.Fatal("fresh Attachment facts changed logical identity or retained exporter")
	}
}

func TestRecipientBindingRefusesSubstitutionAndAuthorityExtension(t *testing.T) {
	proof, original, now := bindingInputs(t)
	changes := map[string]func(*capsule.Request){
		"Network":                     func(v *capsule.Request) { v.Network[0]++ },
		"Target":                      func(v *capsule.Request) { v.Target[0]++ },
		"Publication":                 func(v *capsule.Request) { v.PublicationDigest[0]++ },
		"profile":                     func(v *capsule.Request) { v.ProfileDigest[0]++ },
		"recovery without continuity": func(v *capsule.Request) { v.AttachmentGeneration = 2 },
		"missing Connection nonce":    func(v *capsule.Request) { v.ConnectionNonce = [32]byte{} },
		"capsule deadline":            func(v *capsule.Request) { v.Deadline = now.Add(11 * time.Second) },
		"local maximum":               func(v *capsule.Request) { v.WorkSafetyMaximum++ },
		"recovery after work":         func(v *capsule.Request) { v.NoNewRecoveryAfter = v.WorkSafetyNotAfter + 1 },
		"expired work":                func(v *capsule.Request) { v.WorkSafetyNotAfter = now.Unix() },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			candidate := original
			change(&candidate)
			digest, _ := candidate.Digest()
			if _, err := deriveRecipientBinding(proof, original.ProfileDigest, candidate, digest, now.Add(600*time.Second), now); err == nil {
				t.Fatal("foreign or extended immutable tuple accepted")
			}
		})
	}
	for _, absent := range []publication.Proof{{}} {
		digest, _ := original.Digest()
		if _, err := deriveRecipientBinding(absent, original.ProfileDigest, original, digest, now.Add(600*time.Second), now); err == nil {
			t.Fatal("absent sealed Publication produced a tuple")
		}
	}
	if _, err := deriveRecipientBinding(proof, original.ProfileDigest, original, [32]byte{8}, now.Add(600*time.Second), now); err == nil {
		t.Fatal("supplied foreign plaintext digest accepted")
	}
}

func TestRecipientBindingCannotSupplyOriginalOperation(t *testing.T) {
	proof, request, now := bindingInputs(t)
	digest, err := request.Digest()
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range []*executionruntime.Operation{nil, {}} {
		if binding, err := BindRecipient(t.Context(), operation, proof, request.ProfileDigest, request, digest, now.Add(600*time.Second)); err == nil || binding != nil {
			t.Fatal("public proof supplied live operation")
		}
	}
	binding, err := deriveRecipientBinding(proof, request.ProfileDigest, request, digest, now.Add(600*time.Second), now)
	if err != nil {
		t.Fatal(err)
	}
	if binding.Check(t.Context(), &executionruntime.Operation{}) == nil {
		t.Fatal("tuple alone supplied qualified authority")
	}
}
