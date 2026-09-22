package inspection

import (
	"crypto/ed25519"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
)

// VerifyACA2Corpus verifies one whole ACA2 disclosure and then its fixed Alpha
// Corpus component under the independently pinned corpus authority. Its output
// is inspection data only; an Endpoint must separately retain it in its own
// corpus floor before any resolution use.
func VerifyACA2Corpus(catalogRaw []byte, disclosure, corpusAuthority ed25519.PublicKey, corpusRaw []byte,
	network [32]byte, at time.Time) (*alpha.Corpus, alphacontrol.Outcome) {
	catalog, _, err := alphacontrol.VerifyV2(catalogRaw, disclosure, at)
	if err != nil {
		return nil, alphacontrol.OutcomeInvalid
	}
	return VerifyCorpusComponent(catalog.Components[3], corpusRaw, corpusAuthority, catalog.Cohort, network, at)
}
