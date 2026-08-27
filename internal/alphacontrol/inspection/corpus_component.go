package inspection

import (
	"crypto/ed25519"
	"crypto/sha256"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
)

// VerifyCorpusComponent checks the ACA2 Alpha Corpus component against its
// independently pinned corpus authority. It returns a verified corpus only as
// inspection data; neither it nor its catalog authorizes an Endpoint.
func VerifyCorpusComponent(reference alphacontrol.Component, raw []byte, root ed25519.PublicKey,
	cohort string, network [32]byte, at time.Time) (*alpha.Corpus, alphacontrol.Outcome) {
	if reference.Class != alphacontrol.ComponentCorpus || len(raw) == 0 || uint64(len(raw)) != uint64(reference.Size) ||
		sha256.Sum256(raw) != reference.Digest || len(root) != ed25519.PublicKeySize || sha256.Sum256(root) != reference.RootID ||
		cohort == "" || network == [32]byte{} || at.IsZero() {
		return nil, alphacontrol.OutcomeDigestMismatch
	}
	corpus, err := alpha.OpenCorpus(root, raw)
	if err != nil || corpus.Cohort() != cohort || corpus.Network() != network || corpus.Serial() != reference.Generation ||
		!corpus.NotAfter().Equal(reference.NotAfter) {
		return nil, alphacontrol.OutcomeInvalid
	}
	if err := corpus.ValidAt(at); err != nil {
		if alpha.HasFailure(err, alpha.FailureExpired) || alpha.HasFailure(err, alpha.FailureNotYetValid) {
			return nil, alphacontrol.OutcomeExpired
		}
		return nil, alphacontrol.OutcomeInvalid
	}
	return corpus, alphacontrol.OutcomeAccepted
}
