package alphacontrol

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"
)

func TestInitialCatalogPreparationUsesOrdinaryVerification(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2_000_400_000, 0).UTC()
	input := Catalog{Cohort: "isolated-cohort", Generation: 1, NotBefore: now.Add(-time.Second), NotAfter: now.Add(time.Hour)}
	for i := range input.Components {
		input.Components[i] = Component{Class: ComponentClass(i + 1), RootID: [32]byte{byte(i + 1)}, Generation: 1, NotAfter: now.Add(time.Minute), Size: 100, Digest: [32]byte{byte(i + 4)}}
	}
	message, err := PrepareInitialCatalogSigningInput(input)
	if err != nil {
		t.Fatal(err)
	}
	raw := append(append([]byte(nil), message[len(catalogDomain):]...), ed25519.Sign(private, message)...)
	decoded, _, err := Verify(raw, public, now)
	if err != nil || decoded.Components != input.Components || decoded.Cohort != input.Cohort {
		t.Fatalf("ordinary catalog verification: %v", err)
	}
	raw[6] ^= 1
	if _, _, err := Verify(raw, public, now); err == nil {
		t.Fatal("substituted catalog verified")
	}
	for name, mutate := range map[string]func(*Catalog){
		"later catalog":     func(v *Catalog) { v.Generation = 2 },
		"predecessor":       func(v *Catalog) { v.PreviousDigest[0] = 1 },
		"signed input":      func(v *Catalog) { v.Signature[0] = 1 },
		"later component":   func(v *Catalog) { v.Components[1].Generation = 2 },
		"expired component": func(v *Catalog) { v.Components[1].NotAfter = v.NotBefore },
		"negative time":     func(v *Catalog) { v.NotBefore = time.Unix(-1, 0).UTC() },
		"component order":   func(v *Catalog) { v.Components[0], v.Components[1] = v.Components[1], v.Components[0] },
	} {
		t.Run(name, func(t *testing.T) {
			changed := input
			mutate(&changed)
			if raw, err := PrepareInitialCatalogSigningInput(changed); err == nil || raw != nil {
				t.Fatal("invalid initial catalog produced signing input")
			}
		})
	}
}
