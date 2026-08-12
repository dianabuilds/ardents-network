package state

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"
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

// Verify independently recomputes the authenticated current decision from
// persisted canonical bytes. It does not call the product validation pipeline.
func Verify(input Case) Result {
	if err := validateCase(input); err != nil {
		return Result{Verdict: "invalid", Reason: err.Error()}
	}
	evidence, err := readEvidence(input.Root, input.Materializations)
	if err != nil {
		return Result{Verdict: "invalid", Reason: err.Error()}
	}
	epoch, err := verifyChain(input, evidence)
	if err != nil {
		return Result{Verdict: "fail", Reason: err.Error(), Generation: evidence.current}
	}
	if err := verifyDecision(input, evidence, epoch); err != nil {
		return Result{Verdict: "fail", Reason: err.Error(), Generation: evidence.current}
	}
	return Result{Verdict: "pass", Reason: "independent offline verification passed",
		Generation: evidence.current, Epoch: epoch.number}
}

func validateCase(input Case) error {
	if input.Root == "" {
		return errors.New("evidence root is required")
	}
	if input.Threshold < 1 || input.Threshold > len(input.Authorities) {
		return errors.New("authority threshold is outside the authority set")
	}
	if len(input.Authorities) > 16 || len(input.Materializations) > 64 {
		return errors.New("qualification input exceeds its finite set bounds")
	}
	if input.Now.IsZero() {
		return errors.New("verification time is required")
	}
	for id, public := range input.Authorities {
		if len(public) != ed25519.PublicKeySize {
			return errors.New("authority public key has invalid length")
		}
		if keyID(public) != id {
			return errors.New("authority identifier does not match its public key")
		}
	}
	return nil
}

func verifyDecision(input Case, evidence persistedEvidence, epoch verifiedEpoch) error {
	if fmt.Sprintf("%x", epoch.digest) != evidence.current {
		return errors.New("generation name does not match the epoch digest")
	}
	if len(evidence.inputs) != int(epoch.cutoff) {
		return errors.New("candidate input file count does not match its cutoff")
	}
	if recordRoot(evidence.inputs, 0x10) != epoch.inputRoot {
		return errors.New("candidate input root is inconsistent")
	}
	accepted, rejected := evaluateInputs(input, epoch, evidence.inputs)
	if err := verifyView(epoch, accepted, rejected); err != nil {
		return err
	}
	return verifyMaterials(epoch, accepted, evidence.materializations)
}

func keyID(public []byte) [32]byte { return sha256.Sum256(public) }
