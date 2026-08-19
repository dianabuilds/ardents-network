package namelease

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
)

// TestR044_SignatureAcceptsValidAndRejectsTampered is a feasibility vector:
// an Ed25519 signature over the canonical Record payload is accepted iff the
// payload has not been tampered with. R-044 remains open and this test does not
// select the Stage 6 suite.
func TestR044_SignatureAcceptsValidAndRejectsTampered(t *testing.T) {
	t.Parallel()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	rec := Record{
		Name: "sign.example", Generation: 1, Revision: 1, State: "active",
		Authority: "alice", Target: "t1", ParentName: "",
	}
	if err := rec.sign(priv); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if err := rec.verify(pub); err != nil {
		t.Fatalf("Verify (untouched): %v", err)
	}
	// Tamper: change the Target. The signature must no longer verify.
	rec.Target = "t2"
	if err := rec.verify(pub); err == nil {
		t.Errorf("Verify on tampered Target must fail")
	}
}

// TestR044_SignatureRejectsWrongPublicKey: a signature produced by
// one key is rejected when verified against a different key.
func TestR044_SignatureRejectsWrongPublicKey(t *testing.T) {
	t.Parallel()
	pubA, privA, _ := ed25519.GenerateKey(rand.Reader)
	pubB, _, _ := ed25519.GenerateKey(rand.Reader)
	rec := Record{
		Name: "k.example", Generation: 1, Revision: 1, State: "active",
		Authority: "alice", Target: "t1",
	}
	if err := rec.sign(privA); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if err := rec.verify(pubA); err != nil {
		t.Fatalf("Verify with own key: %v", err)
	}
	if err := rec.verify(pubB); err == nil {
		t.Errorf("Verify with foreign key must fail")
	}
}

// TestR044_SignatureMissingRejected: a Record with no signature
// fails verification.
func TestR044_SignatureMissingRejected(t *testing.T) {
	t.Parallel()
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	rec := Record{Name: "m.example", Generation: 1, Revision: 1, State: "active"}
	if err := rec.verify(pub); err == nil {
		t.Errorf("Verify on unsigned Record must fail")
	}
}

// TestR044_SignatureRejectsGenerationChange per S6.1 DoD: a recorded
// signature must not survive a generation change (replay across
// generations).
func TestR044_SignatureRejectsGenerationChange(t *testing.T) {
	t.Parallel()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	rec := Record{
		Name: "g.example", Generation: 1, Revision: 1, State: "active",
		Authority: "alice", Target: "t1",
	}
	if err := rec.sign(priv); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if err := rec.verify(pub); err != nil {
		t.Fatalf("Verify (gen 1): %v", err)
	}
	// Adversary tries to replay the same signature on a bumped
	// generation: must fail.
	rec.Generation = 2
	if err := rec.verify(pub); err == nil {
		t.Errorf("Verify after generation bump must fail (replay)")
	}
}

// TestR044_SignatureRejectsRevisionChange per S6.1 DoD: a recorded
// signature must not survive a revision change.
func TestR044_SignatureRejectsRevisionChange(t *testing.T) {
	t.Parallel()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	rec := Record{
		Name: "r.example", Generation: 1, Revision: 1, State: "active",
		Authority: "alice", Target: "t1",
	}
	if err := rec.sign(priv); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	rec.Revision = 2
	if err := rec.verify(pub); err == nil {
		t.Errorf("Verify after revision bump must fail")
	}
}
