package alphacontrol_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
)

func TestACA2BindsExactlyFourFixedComponentsWithoutChangingACA1(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2_000_400_000, 0).UTC()
	catalog := alphacontrol.CatalogV2{Cohort: "alpha-one", Generation: 3, NotBefore: now.Add(-time.Second), NotAfter: now.Add(time.Minute)}
	for index := range catalog.Components {
		body := []byte{byte(index + 1)}
		catalog.Components[index] = alphacontrol.Component{Class: alphacontrol.ComponentClass(index + 1), RootID: [32]byte{byte(index + 1)},
			Generation: uint64(index + 1), NotAfter: now.Add(time.Minute), Size: uint32(len(body)), Digest: sha256.Sum256(body)}
	}
	raw, err := signCatalogV2(catalog, private)
	if err != nil {
		t.Fatal(err)
	}
	verified, _, err := alphacontrol.VerifyV2(raw, public, now)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Components[3].Class != alphacontrol.ComponentCorpus || verified.Components[3].Generation != 4 {
		t.Fatalf("ACA2 corpus component = %+v", verified.Components[3])
	}
	if _, _, err := alphacontrol.Verify(raw, public, now); err == nil {
		t.Fatal("ACA1 verifier accepted ACA2 catalog")
	}
	changed := append([]byte(nil), raw...)
	changed[len(changed)-1]++
	if _, _, err := alphacontrol.VerifyV2(changed, public, now); err == nil {
		t.Fatal("changed ACA2 catalog was accepted")
	}
}
