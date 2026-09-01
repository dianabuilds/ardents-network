package endpoint

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
)

func TestEndpointAcceptAlphaCorpusControlPinsACA2AndAdvancesDurableFloor(t *testing.T) {
	now := time.Unix(2_000_400_000, 0).UTC()
	network := targetLinkBytes(1)
	disclosurePublic, disclosurePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	corpusPublic, corpusPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	link, err := alpha.ParseServiceLink("ardents-alpha://blog.alice")
	if err != nil {
		t.Fatal(err)
	}
	corpusRaw, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "alpha-one", Network: network, Serial: 4,
		Bindings: []alpha.BindingInput{{Link: link, Target: targetLinkBytes(33)}}, NotBefore: now.Add(-time.Second), NotAfter: now.Add(time.Minute)}, corpusPrivate)
	if err != nil {
		t.Fatal(err)
	}
	catalog := alphacontrol.CatalogV2{Cohort: "alpha-one", Generation: 1, NotBefore: now.Add(-time.Second), NotAfter: now.Add(time.Minute)}
	for index := range catalog.Components[:3] {
		body := []byte{byte(index + 1)}
		catalog.Components[index] = alphacontrol.Component{Class: alphacontrol.ComponentClass(index + 1), RootID: [32]byte{byte(index + 1)},
			Generation: 1, NotAfter: now.Add(time.Minute), Size: uint32(len(body)), Digest: sha256.Sum256(body)}
	}
	catalog.Components[3] = alphacontrol.Component{Class: alphacontrol.ComponentCorpus, RootID: sha256.Sum256(corpusPublic),
		Generation: 4, NotAfter: now.Add(time.Minute), Size: uint32(len(corpusRaw)), Digest: sha256.Sum256(corpusRaw)}
	catalogRaw, err := signCatalogV2Fixture(catalog, disclosurePrivate)
	if err != nil {
		t.Fatal(err)
	}
	floor, err := alpha.OpenPersistentFloor(alpha.PersistentFloorConfig{Root: t.TempDir(), Authority: corpusPublic, Cohort: "alpha-one", Network: network})
	if err != nil {
		t.Fatal(err)
	}
	defer floor.Close()
	endpoint := &endpoint{network: network}
	accepted, err := endpoint.AcceptAlphaCorpusControl(alphaCorpusControlInput{Catalog: catalogRaw, DisclosureKey: disclosurePublic,
		Corpus: corpusRaw, CorpusAuthority: corpusPublic, Floor: floor, At: now})
	if err != nil || accepted.Serial() != 4 {
		t.Fatalf("accepted alpha corpus = (%v, %v)", accepted, err)
	}
	changed := append([]byte(nil), catalogRaw...)
	changed[len(changed)-1]++
	if result, err := endpoint.AcceptAlphaCorpusControl(alphaCorpusControlInput{Catalog: changed, DisclosureKey: disclosurePublic,
		Corpus: corpusRaw, CorpusAuthority: corpusPublic, Floor: floor, At: now}); err == nil || result != nil {
		t.Fatalf("changed ACA2 acceptance = (%v, %v)", result, err)
	}
}
