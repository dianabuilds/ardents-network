package namespace

import (
	"encoding/hex"
	"errors"
	"time"
)

// OpenClaimWinner verifies one authenticated R-042 close and returns only its
// proven root-claim winner. Admission was consumed when the commitment entered
// the Epoch input log; this function neither receives nor verifies a later
// Gateway admission proof.
func OpenClaimWinner(order ClaimOrder, proof ClaimProof) (*ClaimWinner, error) {
	result, err := order.Verify(proof)
	if err != nil || result.Outcome != "accepted" {
		return nil, errors.New("root claim ordering is not accepted")
	}
	var winner *Claim
	for index := range proof.Claims {
		if proof.Claims[index].Ordinal == result.WinnerOrdinal {
			winner = &proof.Claims[index]
			break
		}
	}
	if winner == nil {
		return nil, errors.New("root claim winner is unavailable")
	}
	return &ClaimWinner{value: &claimWinner{network: proof.Network, name: winner.Name, authority: winner.Authority,
		ordinal: result.WinnerOrdinal, epoch: proof.Epoch}}, nil
}

// Materialize derives one root Record from a previously verified Epoch winner.
// Callers cannot substitute a raw proof, claim ordinal, Name, Authority, or
// lease deadline at this boundary. A successful use consumes the local fact;
// signing and threshold Store.Commit remain separate current-state steps.
func (winner *ClaimWinner) Materialize(current *Record, materializedAt time.Time,
	policy Policy,
) (Record, error) {
	record, err := winner.prepare(current, materializedAt, policy)
	if err != nil {
		return Record{}, err
	}
	if !winner.consume() {
		return Record{}, errors.New("root claim winner was already materialized")
	}
	return record, nil
}

func (winner *ClaimWinner) prepare(current *Record, materializedAt time.Time, policy Policy) (Record, error) {
	if winner == nil || winner.value == nil || materializedAt.IsZero() || materializedAt.Unix() <= 0 {
		return Record{}, errors.New("root claim materialization input is invalid")
	}
	winner.value.mu.Lock()
	defer winner.value.mu.Unlock()
	if winner.value.consumed {
		return Record{}, errors.New("root claim winner was already materialized")
	}
	if current != nil && current.Name != winner.value.name {
		return Record{}, errors.New("root claim predecessor does not match the authenticated winner")
	}
	generation := uint64(1)
	expectedGeneration, expectedRevision := uint64(0), uint64(0)
	if current != nil {
		generation = current.Generation + 1
		expectedGeneration, expectedRevision = current.Generation, current.Revision
	}
	op := Op{Kind: opClaim, Name: winner.value.name, Generation: generation,
		ExpectedGeneration: expectedGeneration, ExpectedRevision: expectedRevision,
		Authority: hex.EncodeToString(winner.value.authority[:])}
	return applyAt(current, materializedAt, op, policy)
}

func (winner *ClaimWinner) consume() bool {
	if winner == nil || winner.value == nil {
		return false
	}
	winner.value.mu.Lock()
	defer winner.value.mu.Unlock()
	if winner.value.consumed {
		return false
	}
	winner.value.consumed = true
	return true
}
