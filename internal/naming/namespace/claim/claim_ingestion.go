package claim

import (
	"crypto/ed25519"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/naming"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/admission"
)

// AdmitClaimCommitment verifies and spends exactly one R-045 root-claim proof
// bound to the hidden R-042 commitment. It is the only local source of the
// AdmissionDigest later carried into an Epoch input.
func AdmitClaimCommitment(gate *admission.Admission, now int64, commitment [32]byte,
	proof admission.Proof,
) (*ClaimCommitment, error) {
	if gate == nil || commitment == [32]byte{} || proof.Challenge.Network != gate.Network() ||
		proof.Challenge.Epoch != gate.Epoch() || proof.Challenge.Surface != "root-claim" ||
		proof.Challenge.OperationDigest != commitment {
		return nil, errors.New("claim commitment admission is invalid")
	}
	if accepted, _ := gate.Verify(now, proof); !accepted {
		return nil, errors.New("claim commitment admission is denied")
	}
	return &ClaimCommitment{network: gate.Network(), epoch: gate.Epoch(), commitment: commitment,
		admission: proof.Digest()}, nil
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

// EpochInput returns the only log representation of a locally admitted
// commitment. Network/Epoch code may order and commit these opaque bytes but
// cannot inspect the local admission proof or the hidden claim reveal.
func (commitment *ClaimCommitment) EpochInput() (EpochClaimInput, error) {
	if commitment == nil || commitment.network == [32]byte{} || commitment.epoch == 0 ||
		commitment.commitment == [32]byte{} || commitment.admission == [32]byte{} {
		return EpochClaimInput{}, errors.New("claim Epoch input is invalid")
	}
	var input EpochClaimInput
	copy(input.raw[:32], commitment.commitment[:])
	copy(input.raw[32:], commitment.admission[:])
	input.commitment = commitment.commitment
	return input, nil
}

// Canonical returns a copy of the fixed opaque Epoch-log input.
func (input EpochClaimInput) Canonical() []byte { return append([]byte(nil), input.raw[:]...) }

// Commitment returns the exact digest bound by the admission proof.
func (input EpochClaimInput) Commitment() [32]byte { return input.commitment }

// InputLeaf returns the domain-separated leaf which the authenticated Epoch
// input root must contain at ordinal. The Epoch log owns ordinal assignment;
// this opaque fact owns only the admitted bytes that are committed there.
func (input EpochClaimInput) InputLeaf(ordinal uint32) [32]byte {
	return epochClaimInputLeaf(ordinal, input.raw)
}

// VerifyClose returns the winner only when the authenticated close includes
// this exact locally admitted input at ordinal. It binds local R-045 admission
// to the threshold proof without making Namespace own the Network Epoch log.
func (input EpochClaimInput) VerifyClose(order ClaimOrder, ordinal uint32,
	proof ClaimProof,
) (*ClaimWinner, error) {
	if !input.valid() {
		return nil, errors.New("claim Epoch input is invalid")
	}
	winner, err := OpenClaimWinner(order, proof)
	if err != nil || winner.value == nil || winner.value.ordinal != ordinal {
		return nil, errors.New("claim Epoch close does not select admitted input")
	}
	for _, claim := range proof.Claims {
		if claim.Ordinal == ordinal {
			if input.InputLeaf(ordinal) == claimInputLeaf(claim) {
				return winner, nil
			}
			break
		}
	}
	return nil, errors.New("claim Epoch close does not include admitted input")
}

func (input EpochClaimInput) valid() bool {
	if input.commitment == [32]byte{} || input.raw == [64]byte{} {
		return false
	}
	var commitment, admission [32]byte
	copy(commitment[:], input.raw[:32])
	copy(admission[:], input.raw[32:])
	return commitment == input.commitment && admission != [32]byte{}
}
