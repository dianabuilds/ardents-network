package state

import (
	"crypto/ed25519"
	"time"

	stateepoch "github.com/dianabuilds/ardents-network/internal/network/epoch"
)

// Case identifies one persisted candidate generation and its supplied evidence.
type Case struct {
	Root             string
	NetworkID        [32]byte
	Authorities      map[[32]byte]ed25519.PublicKey
	Threshold        int
	Now              time.Time
	Materializations [][]byte
}

// Result is the terminal machine outcome of one persisted-state verification.
type Result struct {
	Verdict        string `json:"verdict"`
	Reason         string `json:"reason"`
	Generation     string `json:"generation,omitempty"`
	Epoch          uint64 `json:"epoch,omitempty"`
	EvidenceRoot   string `json:"evidence_root,omitempty"`
	EvidenceDigest string `json:"evidence_digest,omitempty"`
}

// Verify reads one bounded state root and asks the canonical Epoch Module to
// recompute its authenticated current decision.
func Verify(input Case) Result {
	if input.Root == "" || len(input.Materializations) > 64 {
		return Result{Verdict: "invalid", Reason: "bounded evidence root and materializations are required"}
	}
	evidence, err := readEvidence(input.Root, input.Materializations)
	if err != nil {
		return Result{Verdict: "invalid", Reason: err.Error()}
	}
	decision, err := stateepoch.VerifyEvidence(stateepoch.Policy{
		NetworkID: input.NetworkID, Authorities: input.Authorities, Threshold: input.Threshold, Now: input.Now,
	}, evidence.current, evidence.generations, evidence.inputs, evidence.materializations)
	if err != nil {
		return Result{Verdict: "fail", Reason: err.Error(), Generation: evidence.current}
	}
	if decision.Snapshot.Generation != evidence.current {
		return Result{Verdict: "fail", Reason: "epoch report disagrees with current evidence", Generation: evidence.current}
	}
	// Keep the frozen v1 machine reason stable even though Epoch rules now have
	// one canonical implementation instead of a competing qualification copy.
	return Result{Verdict: "pass", Reason: "independent offline verification passed",
		Generation: decision.Snapshot.Generation, Epoch: decision.Snapshot.Epoch}
}
