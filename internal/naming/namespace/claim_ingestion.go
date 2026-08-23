package namespace

import (
	"crypto/ed25519"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/naming"
)

// AdmitClaimCommitment verifies and spends exactly one R-045 root-claim proof
// bound to the hidden R-042 commitment. It is the only local source of the
// AdmissionDigest later carried into an Epoch input.
func AdmitClaimCommitment(admission *Admission, now int64, commitment [32]byte,
	proof Proof,
) (*ClaimCommitment, error) {
	if admission == nil || commitment == [32]byte{} || proof.Challenge.Network != admission.network ||
		proof.Challenge.Epoch != admission.epoch || proof.Challenge.Surface != "root-claim" ||
		proof.Challenge.OperationDigest != commitment {
		return nil, errors.New("claim commitment admission is invalid")
	}
	if accepted, _ := admission.Verify(now, proof); !accepted {
		return nil, errors.New("claim commitment admission is denied")
	}
	return &ClaimCommitment{network: admission.network, epoch: admission.epoch, commitment: commitment,
		admission: challengeDigest(proof.Challenge)}, nil
}

// Reveal opens one locally admitted commitment for Epoch reveal. It derives
// the admission digest; callers cannot supply or substitute it.
func (commitment *ClaimCommitment) Reveal(name string, secret [32]byte, authority [32]byte,
	signature [64]byte,
) (Claim, error) {
	if commitment == nil || commitment.network == [32]byte{} || commitment.epoch == 0 ||
		commitment.commitment == [32]byte{} || commitment.admission == [32]byte{} || authority == [32]byte{} || secret == [32]byte{} {
		return Claim{}, errors.New("claim reveal input is invalid")
	}
	parsed, err := naming.Parse(name)
	if err != nil || string(parsed) != name {
		return Claim{}, errors.New("claim reveal Name is invalid")
	}
	claim := Claim{Name: name, Secret: secret, Authority: authority, Commitment: commitment.commitment,
		AdmissionDigest: commitment.admission, Signature: signature}
	if CommitmentFor(commitment.network, commitment.epoch, claim) != commitment.commitment ||
		!ed25519.Verify(ed25519.PublicKey(authority[:]), RevealTranscript(commitment.network, commitment.epoch, claim), signature[:]) {
		return Claim{}, errors.New("claim reveal does not open admitted commitment")
	}
	return claim, nil
}
