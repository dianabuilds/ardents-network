package stage6verify_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/lab/stage6verify"
)

func TestStage6VerifierRejectsMissingResolutionRoleView(t *testing.T) {
	base := t.TempDir()
	writeEvidenceCampaign(t, base, "source-commit", "clean")
	mutations := map[string]func(string) error{
		"missing Relay role":         removeResolutionRelayView,
		"changed Namespace proof":    changeResolutionNamespaceProof,
		"changed Record corpus":      changeResolutionRecordCorpus,
		"changed deep proof":         changeDeepNamespaceProof,
		"changed deep Record corpus": changeDeepNamespaceRecordCorpus,
		"changed deep proof size":    changeDeepNamespaceProofSize,
		"changed transition corpus":  changeResolutionTransitionCorpus,
		"changed rejection corpus":   changeResolutionRejectionCorpus,
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := cloneBundle(base, root); err != nil {
				t.Fatal(err)
			}
			if err := mutate(root); err != nil {
				t.Fatal(err)
			}
			verdict := (stage6verify.Stage6Verifier{}).Verify(filepath.Join(root, "manifest"),
				filepath.Join(root, "evidence"), filepath.Join(root, "private"), filepath.Join(root, "verdict"))
			if verdict.Status != "fail" || len(verdict.Diagnostics) != 1 || verdict.Diagnostics[0] != "D0:predicate-false" {
				t.Fatalf("verdict=%+v", verdict)
			}
		})
	}
}

func changeDeepNamespaceProof(root string) error {
	return rewriteResolutionEvidence(root, func(evidence map[string]json.RawMessage) error {
		var proof []byte
		if err := json.Unmarshal(evidence["DeepNamespaceProof"], &proof); err != nil || len(proof) == 0 {
			return os.ErrInvalid
		}
		proof[len(proof)-1] ^= 1
		var err error
		evidence["DeepNamespaceProof"], err = json.Marshal(proof)
		return err
	})
}

func changeDeepNamespaceRecordCorpus(root string) error {
	return rewriteResolutionEvidence(root, func(evidence map[string]json.RawMessage) error {
		var records [][]byte
		if err := json.Unmarshal(evidence["DeepNamespaceRecords"], &records); err != nil ||
			len(records) == 0 || len(records[0]) == 0 {
			return os.ErrInvalid
		}
		records[0][len(records[0])-1] ^= 1
		var err error
		evidence["DeepNamespaceRecords"], err = json.Marshal(records)
		return err
	})
}

func changeDeepNamespaceProofSize(root string) error {
	return rewriteResolutionEvidence(root, func(evidence map[string]json.RawMessage) error {
		var size uint32
		if err := json.Unmarshal(evidence["DeepProofBytes"], &size); err != nil {
			return err
		}
		var err error
		evidence["DeepProofBytes"], err = json.Marshal(size + 1)
		return err
	})
}

func changeResolutionRejectionCorpus(root string) error {
	return rewriteResolutionEvidence(root, func(evidence map[string]json.RawMessage) error {
		var rejections []struct {
			Ordinal    uint32
			Commitment [32]byte
			Reason     string
		}
		if err := json.Unmarshal(evidence["NamespaceClaimRejections"], &rejections); err != nil {
			return err
		}
		if len(rejections) == 0 {
			return os.ErrInvalid
		}
		rejections[0].Reason = "changed"
		var err error
		evidence["NamespaceClaimRejections"], err = json.Marshal(rejections)
		return err
	})
}

func changeResolutionTransitionCorpus(root string) error {
	return rewriteResolutionEvidence(root, func(evidence map[string]json.RawMessage) error {
		var transitions [][]byte
		if err := json.Unmarshal(evidence["NamespaceTransitions"], &transitions); err != nil {
			return err
		}
		if len(transitions) == 0 || len(transitions[0]) == 0 {
			return os.ErrInvalid
		}
		transitions[0][len(transitions[0])-1] ^= 1
		var err error
		evidence["NamespaceTransitions"], err = json.Marshal(transitions)
		return err
	})
}

func changeResolutionRecordCorpus(root string) error {
	return rewriteResolutionEvidence(root, func(evidence map[string]json.RawMessage) error {
		var records [][]byte
		if err := json.Unmarshal(evidence["NamespaceRecords"], &records); err != nil {
			return err
		}
		if len(records) == 0 || len(records[0]) == 0 {
			return os.ErrInvalid
		}
		records[0][len(records[0])-1] ^= 1
		var err error
		evidence["NamespaceRecords"], err = json.Marshal(records)
		return err
	})
}

func removeResolutionRelayView(root string) error {
	return rewriteResolutionEvidence(root, func(evidence map[string]json.RawMessage) error {
		delete(evidence, "Relays")
		return nil
	})
}

func changeResolutionNamespaceProof(root string) error {
	return rewriteResolutionEvidence(root, func(evidence map[string]json.RawMessage) error {
		var proof []byte
		if err := json.Unmarshal(evidence["NamespaceProof"], &proof); err != nil {
			return err
		}
		proof[len(proof)-1] ^= 1
		var err error
		evidence["NamespaceProof"], err = json.Marshal(proof)
		return err
	})
}

func rewriteResolutionEvidence(root string, mutate func(map[string]json.RawMessage) error) error {
	const ordinal = 20
	tracePath := filepath.Join(root, "evidence", "cells", "20", "observations", "trace.jsonl")
	raw, err := os.ReadFile(tracePath)
	if err != nil {
		return err
	}
	var trace mutationTrace
	if err = json.Unmarshal(raw, &trace); err != nil {
		return err
	}
	var evidence map[string]json.RawMessage
	if err = json.Unmarshal(trace.Auxiliary, &evidence); err != nil {
		return err
	}
	if err = mutate(evidence); err != nil {
		return err
	}
	if trace.Auxiliary, err = json.Marshal(evidence); err != nil {
		return err
	}
	if raw, err = json.Marshal(trace); err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err = os.WriteFile(tracePath, raw, 0o600); err != nil {
		return err
	}
	indexPath := filepath.Join(root, "evidence", "index.json")
	indexRaw, err := os.ReadFile(indexPath)
	if err != nil {
		return err
	}
	var index mutationIndex
	if err = json.Unmarshal(indexRaw, &index); err != nil {
		return err
	}
	digest := sha256.Sum256(raw)
	index.Cells[ordinal].Streams[0].Size = int64(len(raw))
	index.Cells[ordinal].Streams[0].SHA256 = hex.EncodeToString(digest[:])
	if indexRaw, err = json.Marshal(index); err != nil {
		return err
	}
	return os.WriteFile(indexPath, indexRaw, 0o600)
}
