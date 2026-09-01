package alphacontrol_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
)

func TestInspectRequiresCatalogBindingFloorsAndComponentVerification(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2_000_400_000, 0).UTC()
	components, roots, catalog := signedFixture(t, now)
	raw, err := signCatalog(catalog, private)
	if err != nil {
		t.Fatal(err)
	}
	result, floor, err := alphacontrol.Inspect(raw, public, roots, components, alphacontrol.Floor{}, now,
		func(component alphacontrol.Component, statement alphacontrol.ComponentStatement, at time.Time) alphacontrol.Outcome {
			if len(statement.Body) == 0 || at.IsZero() || component.RootID == [32]byte{} {
				return alphacontrol.OutcomeInvalid
			}
			return alphacontrol.OutcomeAccepted
		})
	if err != nil || result.Catalog != alphacontrol.OutcomeAccepted || floor.CatalogGeneration != 1 {
		t.Fatalf("Inspect = %+v, %+v, %v", result, floor, err)
	}
	for _, component := range result.Components {
		if component.Outcome != alphacontrol.OutcomeAccepted {
			t.Fatalf("component result = %+v", component)
		}
	}
	changed := components
	changed[0] = []byte("changed")
	result, _, err = alphacontrol.Inspect(raw, public, roots, changed, floor, now, func(alphacontrol.Component, alphacontrol.ComponentStatement, time.Time) alphacontrol.Outcome {
		return alphacontrol.OutcomeAccepted
	})
	if err != nil || result.Components[0].Outcome != alphacontrol.OutcomeDigestMismatch {
		t.Fatalf("changed component result = %+v, %v", result, err)
	}
	conflict := catalog
	conflict.Components[0].Generation = 1
	conflict.Components[0].Digest = sha256.Sum256([]byte("different"))
	conflictingRaw, err := signCatalog(conflict, private)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = alphacontrol.Inspect(conflictingRaw, public, roots, components, floor, now, func(alphacontrol.Component, alphacontrol.ComponentStatement, time.Time) alphacontrol.Outcome {
		return alphacontrol.OutcomeAccepted
	})
	if err == nil || result.Catalog != alphacontrol.OutcomeConflict {
		t.Fatalf("same-generation catalog component conflict = %+v, %v", result, err)
	}
}

func TestVerifyRejectsChangedSignedPayloadAndExpiry(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2_000_400_000, 0).UTC()
	raw, err := signCatalog(catalogFixture(now, [3][]byte{[]byte("a"), []byte("b"), []byte("c")}), private)
	if err != nil {
		t.Fatal(err)
	}
	changed := append([]byte(nil), raw...)
	changed[7]++
	if _, _, err := alphacontrol.Verify(changed, public, now); err == nil {
		t.Fatal("changed catalog payload was accepted")
	}
	if _, _, err := alphacontrol.Verify(raw, public, now.Add(2*time.Minute)); err == nil {
		t.Fatal("expired catalog was accepted")
	}
	if _, _, err := alphacontrol.Verify(make([]byte, alphacontrol.MaximumCatalogSize+1), public, now); err == nil {
		t.Fatal("oversized catalog was accepted")
	}
}

func TestInspectReportsUnavailableAndRefusesCatalogRollback(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2_000_400_000, 0).UTC()
	components, roots, catalog := signedFixture(t, now)
	catalog.Generation = 2
	raw, err := signCatalog(catalog, private)
	if err != nil {
		t.Fatal(err)
	}
	unavailable := components
	unavailable[1] = nil
	result, floor, err := alphacontrol.Inspect(raw, public, roots, unavailable, alphacontrol.Floor{}, now,
		func(alphacontrol.Component, alphacontrol.ComponentStatement, time.Time) alphacontrol.Outcome {
			return alphacontrol.OutcomeAccepted
		})
	if err != nil || result.Components[1].Outcome != alphacontrol.OutcomeUnavailable || floor.CatalogGeneration != 2 {
		t.Fatalf("unavailable component inspection = %+v, %+v, %v", result, floor, err)
	}
	catalog.Generation = 1
	rollback, err := signCatalog(catalog, private)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err = alphacontrol.Inspect(rollback, public, roots, components, floor, now,
		func(alphacontrol.Component, alphacontrol.ComponentStatement, time.Time) alphacontrol.Outcome {
			return alphacontrol.OutcomeAccepted
		})
	if err == nil || result.Catalog != alphacontrol.OutcomeLowerFloor {
		t.Fatalf("catalog rollback result = %+v, %v", result, err)
	}
}

func TestVerifyComponentRequiresItsOwnSignatureAndCatalogReference(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2_000_400_000, 0).UTC()
	raw, err := signComponent(alphacontrol.ComponentStatement{Class: alphacontrol.ComponentRelease, Generation: 4,
		NotBefore: now.Add(-time.Second), NotAfter: now.Add(time.Minute), Body: []byte("TUF inputs")}, private)

	if err != nil {
		t.Fatal(err)
	}
	reference := alphacontrol.Component{Class: alphacontrol.ComponentRelease, Generation: 4, NotAfter: now.Add(time.Minute),
		RootID: sha256.Sum256(public), Size: uint32(len(raw)), Digest: sha256.Sum256(raw)}
	if outcome := alphacontrol.VerifyComponent(reference, raw, public, now); outcome != alphacontrol.OutcomeAccepted {
		t.Fatalf("VerifyComponent = %q", outcome)
	}
	raw[len(raw)-1]++
	if outcome := alphacontrol.VerifyComponent(reference, raw, public, now); outcome != alphacontrol.OutcomeDigestMismatch {
		t.Fatalf("changed component outcome = %q", outcome)
	}
}

func TestInspectRejectsCatalogSelectedComponentSignerOutsidePinnedRoot(t *testing.T) {
	disclosurePublic, disclosurePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2_000_400_000, 0).UTC()
	components, roots, catalog := signedFixture(t, now)
	_, unpinnedPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	components[0], err = signComponent(alphacontrol.ComponentStatement{Class: alphacontrol.ComponentRelease, Generation: 1,
		NotBefore: now.Add(-time.Second), NotAfter: now.Add(time.Minute), Body: []byte("catalog-selected signer")}, unpinnedPrivate)

	if err != nil {
		t.Fatal(err)
	}
	catalog.Components[0].Size = uint32(len(components[0]))
	catalog.Components[0].Digest = sha256.Sum256(components[0])
	raw, err := signCatalog(catalog, disclosurePrivate)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := alphacontrol.Inspect(raw, disclosurePublic, roots, components, alphacontrol.Floor{}, now,
		func(alphacontrol.Component, alphacontrol.ComponentStatement, time.Time) alphacontrol.Outcome {
			return alphacontrol.OutcomeAccepted
		})
	if err != nil || result.Catalog != alphacontrol.OutcomeAccepted || result.Components[0].Outcome != alphacontrol.OutcomeInvalid {
		t.Fatalf("catalog-selected component root result = %+v, %v", result, err)
	}
}

func TestReaderPersistsCatalogFloorAndRefusesConflictingReplacement(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2_000_400_000, 0).UTC()
	components, roots, catalog := signedFixture(t, now)
	raw, err := signCatalog(catalog, private)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	open := func() *alphacontrol.Reader {
		reader, openErr := alphacontrol.OpenReader(alphacontrol.ReaderConfig{Root: root, DisclosureKey: public, ComponentKeys: roots, Clock: func() time.Time { return now }})
		if openErr != nil {
			t.Fatal(openErr)
		}
		return reader
	}
	reader := open()
	if _, err := reader.Inspect(raw, components, func(alphacontrol.Component, alphacontrol.ComponentStatement, time.Time) alphacontrol.Outcome {
		return alphacontrol.OutcomeAccepted
	}); err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	reader = open()
	defer reader.Close()
	conflict := catalog
	conflict.Components[0].Digest = sha256.Sum256([]byte("different"))
	conflictingRaw, signErr := signCatalog(conflict, private)
	if signErr != nil {
		t.Fatal(signErr)
	}
	if _, inspectErr := reader.Inspect(conflictingRaw, components, func(alphacontrol.Component, alphacontrol.ComponentStatement, time.Time) alphacontrol.Outcome {
		return alphacontrol.OutcomeAccepted
	}); inspectErr == nil {
		t.Fatal("reader accepted a conflicting catalog after restart")
	}
}

func TestReaderLeaseRejectsConcurrentOpenAndAllowsRestart(t *testing.T) {
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, componentPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	componentPublic := componentPrivate.Public().(ed25519.PublicKey)
	var roots [3]ed25519.PublicKey
	for index := range roots {
		roots[index] = componentPublic
	}
	root := t.TempDir()
	config := alphacontrol.ReaderConfig{Root: root, DisclosureKey: public, ComponentKeys: roots, Clock: func() time.Time { return time.Unix(2_000_400_000, 0).UTC() }}
	first, err := alphacontrol.OpenReader(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := alphacontrol.OpenReader(config); err == nil {
		t.Fatal("concurrent reader open was accepted")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := alphacontrol.OpenReader(config)
	if err != nil {
		t.Fatalf("reader restart after released lease: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
}

func signedFixture(t *testing.T, now time.Time) ([3][]byte, [3]ed25519.PublicKey, alphacontrol.Catalog) {
	t.Helper()
	var components [3][]byte
	var roots [3]ed25519.PublicKey
	catalog := alphacontrol.Catalog{Cohort: "alpha-one", Generation: 1, NotBefore: now.Add(-time.Second), NotAfter: now.Add(time.Minute)}
	for index := range components {
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		components[index], err = signComponent(alphacontrol.ComponentStatement{Class: alphacontrol.ComponentClass(index + 1), Generation: 1,
			NotBefore: now.Add(-time.Second), NotAfter: now.Add(time.Minute), Body: []byte{byte(index + 1)}}, private)

		if err != nil {
			t.Fatal(err)
		}
		roots[index] = public
		catalog.Components[index] = alphacontrol.Component{Class: alphacontrol.ComponentClass(index + 1), RootID: sha256.Sum256(public), Generation: 1,
			NotAfter: now.Add(time.Minute), Size: uint32(len(components[index])), Digest: sha256.Sum256(components[index])}
	}
	return components, roots, catalog
}

func catalogFixture(now time.Time, values [3][]byte) alphacontrol.Catalog {
	result := alphacontrol.Catalog{Cohort: "alpha-one", Generation: 1, NotBefore: now.Add(-time.Second), NotAfter: now.Add(time.Minute)}
	for index, value := range values {
		result.Components[index] = alphacontrol.Component{Class: alphacontrol.ComponentClass(index + 1), RootID: [32]byte{byte(index + 1)},
			Generation: 1, NotAfter: now.Add(time.Minute), Size: uint32(len(value)), Digest: sha256.Sum256(value)}
	}
	return result
}
