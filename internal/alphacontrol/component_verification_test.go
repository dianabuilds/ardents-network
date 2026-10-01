package alphacontrol

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"testing"
	"time"
)

// TestVerifiedComponentRequiresItsOwnSignatureAndCatalogReference pins the
// live inspection-path component verifier (inspect.go calls verifiedComponent
// directly; the exported wrapper was retired under ADR-0113).
func TestVerifiedComponentRequiresItsOwnSignatureAndCatalogReference(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2_000_400_000, 0).UTC()
	statement := ComponentStatement{Class: ComponentRelease, Generation: 4,
		NotBefore: now.Add(-time.Second), NotAfter: now.Add(time.Minute), Body: []byte("TUF inputs")}
	payload := append([]byte("ACS1"), 1, byte(statement.Class))
	payload = binary.BigEndian.AppendUint64(payload, statement.Generation)
	payload = binary.BigEndian.AppendUint64(payload, uint64(statement.NotBefore.Unix()))
	payload = binary.BigEndian.AppendUint64(payload, uint64(statement.NotAfter.Unix()))
	payload = binary.BigEndian.AppendUint32(payload, uint32(len(statement.Body)))
	payload = append(payload, statement.Body...)
	raw := append(payload, ed25519.Sign(private, append([]byte("ardents-alpha-control-component-v1\x00"), payload...))...)
	reference := Component{Class: ComponentRelease, Generation: 4, NotAfter: now.Add(time.Minute),
		RootID: sha256.Sum256(public), Size: uint32(len(raw)), Digest: sha256.Sum256(raw)}
	if decoded, outcome := verifiedComponent(reference, raw, public, now); outcome != OutcomeAccepted {
		t.Fatalf("verifiedComponent = %q, %v", outcome, decoded)
	}
	raw[len(raw)-1]++
	if _, outcome := verifiedComponent(reference, raw, public, now); outcome != OutcomeDigestMismatch {
		t.Fatalf("changed component outcome = %q", outcome)
	}
}
