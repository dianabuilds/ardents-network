package endpoint

import (
	"crypto/ed25519"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/alphacontrol/inspection"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
)

// AlphaCorpusControlInput is one explicitly supplied ACA2/corpus pair. It
// contains no URL, downloader, catalog-selected key, or Endpoint readiness
// decision.
type alphaCorpusControlInput struct {
	Catalog         []byte
	DisclosureKey   ed25519.PublicKey
	Corpus          []byte
	CorpusAuthority ed25519.PublicKey
	Floor           *alpha.PersistentFloor
	At              time.Time
}

// AcceptAlphaCorpusControl verifies the closed ACA2 corpus component against
// independent pinned roots, then advances only the supplied Endpoint-local
// floor. It never accepts ACA1, starts a connection, or treats alpha names as
// canonical Namespace claims.
func (endpoint *endpoint) AcceptAlphaCorpusControl(input alphaCorpusControlInput) (*alpha.Corpus, error) {
	if endpoint == nil || input.Floor == nil || input.At.IsZero() {
		return nil, errors.New("alpha corpus control input is incomplete")
	}
	corpus, outcome := inspection.VerifyACA2Corpus(input.Catalog, input.DisclosureKey, input.CorpusAuthority,
		input.Corpus, endpoint.network, input.At)
	if outcome != alphacontrol.OutcomeAccepted || corpus == nil {
		return nil, errors.New("alpha corpus control was not accepted")
	}
	if err := input.Floor.Observe(corpus); err != nil {
		return nil, err
	}
	return corpus, nil
}
