package stage6verify_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/lab/stage6verify"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
)

func TestStage6VerifierRejectsPrivateControlMutations(t *testing.T) {
	mutations := map[string]func(string) error{
		"missing-shape":             removeFinalControlShape,
		"operation-digest-mismatch": corruptControlOperation,
		"rebound-result-state":      corruptControlResultState,
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeEvidenceCampaign(t, root, "source-commit", "clean")
			if err := mutate(root); err != nil {
				t.Fatal(err)
			}
			verdict := (stage6verify.Stage6Verifier{}).Verify(filepath.Join(root, "manifest"),
				filepath.Join(root, "evidence"), filepath.Join(root, "private"), filepath.Join(root, "verdict"))
			if verdict.Status != "fail" || len(verdict.Diagnostics) != 1 ||
				verdict.Diagnostics[0] != "D2:predicate-false" {
				t.Fatalf("verdict=%+v", verdict)
			}
		})
	}
}

type controlMutationEvidence struct {
	Network         [32]byte
	Exchanges       []json.RawMessage
	RelayRequests   uint32
	GatewayRequests uint32
	GatewayAccepted uint32
}

func removeFinalControlShape(root string) error {
	return mutateControlEvidence(root, func(_ *mutationTrace, control *controlMutationEvidence) error {
		control.Exchanges = control.Exchanges[:len(control.Exchanges)-1]
		return nil
	})
}

func corruptControlOperation(root string) error {
	return mutateControlEvidence(root, func(_ *mutationTrace, control *controlMutationEvidence) error {
		var exchange map[string]json.RawMessage
		if err := json.Unmarshal(control.Exchanges[0], &exchange); err != nil {
			return err
		}
		var operation map[string]json.RawMessage
		if err := json.Unmarshal(exchange["Operation"], &operation); err != nil {
			return err
		}
		operation["name"] = json.RawMessage(`"changed.example"`)
		var err error
		exchange["Operation"], err = json.Marshal(operation)
		if err == nil {
			control.Exchanges[0], err = json.Marshal(exchange)
		}
		return err
	})
}

func corruptControlResultState(root string) error {
	return mutateControlEvidence(root, func(trace *mutationTrace, control *controlMutationEvidence) error {
		wires, err := unpackMutationRecords(trace.Output)
		if err != nil {
			return err
		}
		complete := 10
		oldWire := append([]byte(nil), wires[complete]...)
		record, err := namespace.DecodeRecord(wires[complete])
		if err != nil {
			return err
		}
		record.Authority = strings.Repeat("01", 32)
		wires[complete], err = namespace.EncodeRecord(record)
		if err != nil {
			return err
		}
		trace.Output = packMutationRecords(wires)
		oldJSON, _ := json.Marshal(oldWire)
		newJSON, _ := json.Marshal(wires[complete])
		changed := bytes.Replace(control.Exchanges[complete], oldJSON, newJSON, 1)
		if bytes.Equal(changed, control.Exchanges[complete]) {
			return os.ErrInvalid
		}
		control.Exchanges[complete] = changed
		return nil
	})
}

func mutateControlEvidence(root string, mutate func(*mutationTrace, *controlMutationEvidence) error) error {
	tracePath := filepath.Join(root, "evidence", "cells", "22", "observations", "trace.jsonl")
	raw, err := os.ReadFile(tracePath)
	if err != nil {
		return err
	}
	var trace mutationTrace
	if err = json.Unmarshal(raw, &trace); err != nil {
		return err
	}
	var control controlMutationEvidence
	if err = json.Unmarshal(trace.Auxiliary, &control); err != nil {
		return err
	}
	if err = mutate(&trace, &control); err != nil {
		return err
	}
	if trace.Auxiliary, err = json.Marshal(control); err != nil {
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
	index.Cells[22].Streams[0].Size = int64(len(raw))
	index.Cells[22].Streams[0].SHA256 = hex.EncodeToString(digest[:])
	if indexRaw, err = json.Marshal(index); err != nil {
		return err
	}
	return os.WriteFile(indexPath, indexRaw, 0o600)
}

func unpackMutationRecords(raw []byte) ([][]byte, error) {
	if len(raw) < 2 {
		return nil, os.ErrInvalid
	}
	count, offset := int(binary.BigEndian.Uint16(raw)), 2
	result := make([][]byte, count)
	for index := range result {
		if len(raw)-offset < 4 {
			return nil, os.ErrInvalid
		}
		size := int(binary.BigEndian.Uint32(raw[offset:]))
		offset += 4
		if size <= 0 || len(raw)-offset < size {
			return nil, os.ErrInvalid
		}
		result[index] = append([]byte(nil), raw[offset:offset+size]...)
		offset += size
	}
	if offset != len(raw) {
		return nil, os.ErrInvalid
	}
	return result, nil
}

func packMutationRecords(records [][]byte) []byte {
	out := binary.BigEndian.AppendUint16(nil, uint16(len(records)))
	for _, record := range records {
		out = binary.BigEndian.AppendUint32(out, uint32(len(record)))
		out = append(out, record...)
	}
	return out
}
