package alphacontrol

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"testing"
	"time"
)

func TestInitialComponentPreparationUsesOrdinaryVerification(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2_000_400_000, 0).UTC()
	input := ComponentStatement{Class: ComponentRelease, Generation: 1, NotBefore: now.Add(-time.Second), NotAfter: now.Add(time.Minute), Body: []byte("unsigned evidence")}
	message, err := PrepareInitialComponentSigningInput(input)
	if err != nil {
		t.Fatal(err)
	}
	raw := append(append([]byte(nil), message[len(componentDomain):]...), ed25519.Sign(private, message)...)
	reference := Component{Class: input.Class, Generation: 1, NotAfter: input.NotAfter, RootID: sha256.Sum256(public), Size: uint32(len(raw)), Digest: sha256.Sum256(raw)}
	decoded, outcome := verifiedComponent(reference, raw, public, now)
	if outcome != OutcomeAccepted || string(decoded.Body) != string(input.Body) {
		t.Fatalf("ordinary verification: %s", outcome)
	}
	raw[6] ^= 1
	reference.Digest = sha256.Sum256(raw)
	if _, outcome := verifiedComponent(reference, raw, public, now); outcome != OutcomeInvalid {
		t.Fatalf("substituted statement: %s", outcome)
	}
	for name, mutate := range map[string]func(*ComponentStatement){
		"later generation":     func(v *ComponentStatement) { v.Generation = 2 },
		"existing signature":   func(v *ComponentStatement) { v.Signature[0] = 1 },
		"negative timestamp":   func(v *ComponentStatement) { v.NotBefore = time.Unix(-1, 0).UTC() },
		"fractional timestamp": func(v *ComponentStatement) { v.NotBefore = v.NotBefore.Add(time.Nanosecond) },
		"empty body":           func(v *ComponentStatement) { v.Body = nil },
		"unknown class":        func(v *ComponentStatement) { v.Class = 4 },
	} {
		t.Run(name, func(t *testing.T) {
			changed := input
			mutate(&changed)
			if value, err := PrepareInitialComponentSigningInput(changed); err == nil || value != nil {
				t.Fatal("invalid initial statement produced signing input")
			}
		})
	}
}
