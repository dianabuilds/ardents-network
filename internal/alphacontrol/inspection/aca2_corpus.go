package inspection

import (
	"context"
	"crypto/ed25519"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/enrollment"
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

func inspectCorpus(ctx context.Context, config CorpusConfig) (CorpusReport, error) {
	if ctx == nil {
		return CorpusReport{}, errors.New("alpha corpus inspection context is nil")
	}
	control, err := Inspect(ctx, config.Control)
	if err != nil {
		return CorpusReport{Control: control}, err
	}
	if control.Inspection.Catalog != alphacontrol.OutcomeAccepted || control.NetworkID == [32]byte{} {
		return CorpusReport{Control: control}, errors.New("alpha corpus inspection control is not accepted")
	}
	for _, component := range control.Inspection.Components {
		if component.Outcome != alphacontrol.OutcomeAccepted {
			return CorpusReport{Control: control}, errors.New("alpha corpus inspection component is not accepted")
		}
	}
	request := config.Control.Enrollment
	request.ReferenceTime = config.Control.At.UTC()
	verified, err := enrollment.Verify(request)
	if err != nil || len(verified.DisclosureRoot) != ed25519.PublicKeySize || len(verified.CorpusAuthority) != ed25519.PublicKeySize {
		return CorpusReport{Control: control}, errors.New("alpha corpus inspection enrollment roots are invalid")
	}
	disclosure := ed25519.PublicKey(append([]byte(nil), verified.DisclosureRoot...))
	authority := ed25519.PublicKey(append([]byte(nil), verified.CorpusAuthority...))
	corpus, outcome := VerifyACA2Corpus(config.Catalog, disclosure, authority, config.Corpus, control.NetworkID, config.Control.At.UTC())
	if outcome != alphacontrol.OutcomeAccepted || corpus == nil {
		return CorpusReport{Control: control}, errors.New("alpha corpus inspection ACA2 corpus is not accepted")
	}
	return CorpusReport{Control: control, Corpus: corpus, CorpusAuthority: authority}, nil
}
