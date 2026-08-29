package claim

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace/record"
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
		ordinal: result.WinnerOrdinal, epoch: proof.Epoch, close: sha256.Sum256(StatementTranscript(proof))}}, nil
}

// Materialize derives one root Record from a previously verified Epoch winner.
// Callers cannot substitute a raw proof, claim ordinal, Name, Authority, or
// lease deadline at this boundary. A successful use consumes the local fact;
// signing and threshold Store.Commit remain separate current-state steps.
func (winner *ClaimWinner) Materialize(current *record.Record, materializedAt time.Time,
	policy record.Policy,
) (record.Record, error) {
	materialized, err := winner.prepare(current, materializedAt, policy)
	if err != nil {
		return record.Record{}, err
	}
	if !winner.consume() {
		return record.Record{}, errors.New("root claim winner was already materialized")
	}
	return materialized, nil
}

func (winner *ClaimWinner) prepare(current *record.Record, materializedAt time.Time, policy record.Policy) (record.Record, error) {
	if winner == nil || winner.value == nil || materializedAt.IsZero() || materializedAt.Unix() <= 0 {
		return record.Record{}, errors.New("root claim materialization input is invalid")
	}
	winner.value.mu.Lock()
	defer winner.value.mu.Unlock()
	if winner.value.consumed {
		return record.Record{}, errors.New("root claim winner was already materialized")
	}
	if current != nil && current.Name != winner.value.name {
		return record.Record{}, errors.New("root claim predecessor does not match the authenticated winner")
	}
	generation := uint64(1)
	expectedGeneration, expectedRevision := uint64(0), uint64(0)
	if current != nil {
		generation = current.Generation + 1
		expectedGeneration, expectedRevision = current.Generation, current.Revision
	}
	op := record.Op{Kind: "claim", Name: winner.value.name, Generation: generation,
		ExpectedGeneration: expectedGeneration, ExpectedRevision: expectedRevision,
		Authority: hex.EncodeToString(winner.value.authority[:])}
	return record.ApplyAtLegacy(current, materializedAt, op, policy)
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

// Name returns the authenticated root Name selected by the close.
func (winner *ClaimWinner) Name() string {
	if winner == nil || winner.value == nil {
		return ""
	}
	return winner.value.name
}

// BelongsTo reports whether this winner was authenticated for Network and Epoch.
func (winner *ClaimWinner) BelongsTo(network [32]byte, epoch uint64) bool {
	return winner != nil && winner.value != nil && winner.value.network == network && winner.value.epoch == epoch
}

// CloseDigest is the exact authenticated close statement which selected this
// winner. Epoch materialization must bind it as its authenticated Epoch digest.
func (winner *ClaimWinner) CloseDigest() [32]byte {
	if winner == nil || winner.value == nil {
		return [32]byte{}
	}
	return winner.value.close
}

// MaterializeSigned derives and seals the winner's sole Record for its own
// Network and Epoch. The winner is consumed only after signing succeeds.
func (winner *ClaimWinner) MaterializeSigned(current *record.Record, materializedAt time.Time,
	policy record.Policy, signer record.RecordSigner,
) (record.Record, []byte, error) {
	if winner == nil || winner.value == nil {
		return record.Record{}, nil, errors.New("root claim winner is unavailable")
	}
	materialized, err := winner.prepare(current, materializedAt, policy)
	if err != nil {
		return record.Record{}, nil, err
	}
	signed, err := record.SignWith(winner.value.network, materialized, signer)
	if err != nil {
		return record.Record{}, nil, err
	}
	if !winner.consume() {
		return record.Record{}, nil, errors.New("root claim winner was already materialized")
	}
	return materialized, signed, nil
}
