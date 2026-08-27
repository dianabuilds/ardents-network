package inspection_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/alphacontrol/inspection"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
)

func TestVerifyCorpusComponentRequiresTheIndependentCorpusAuthority(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	link, err := alpha.ParseServiceLink("ardents-alpha://blog.alice")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2_000_400_000, 0).UTC()
	network := [32]byte{1}
	raw, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "alpha-one", Network: network, Serial: 7,
		Bindings: []alpha.BindingInput{{Link: link, Target: [32]byte{9}}}, NotBefore: now.Add(-time.Second), NotAfter: now.Add(time.Minute)}, private)
	if err != nil {
		t.Fatal(err)
	}
	reference := alphacontrol.Component{Class: alphacontrol.ComponentCorpus, RootID: sha256.Sum256(public), Generation: 7,
		NotAfter: now.Add(time.Minute), Size: uint32(len(raw)), Digest: sha256.Sum256(raw)}
	corpus, outcome := inspection.VerifyCorpusComponent(reference, raw, public, "alpha-one", network, now)
	if outcome != alphacontrol.OutcomeAccepted || corpus == nil {
		t.Fatalf("corpus component = (%v, %q)", corpus, outcome)
	}
	changed := append([]byte(nil), raw...)
	changed[len(changed)-1]++
	if corpus, outcome := inspection.VerifyCorpusComponent(reference, changed, public, "alpha-one", network, now); corpus != nil || outcome != alphacontrol.OutcomeDigestMismatch {
		t.Fatalf("changed corpus component = (%v, %q)", corpus, outcome)
	}
}
