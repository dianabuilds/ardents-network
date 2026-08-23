package resolution

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace/admission"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/authority"
)

const controlSchema = "ardents-private-name-control-v2"

type controlRequestWire struct {
	Schema    string          `json:"schema"`
	Network   [32]byte        `json:"network"`
	Nonce     [32]byte        `json:"nonce"`
	Deadline  int64           `json:"deadline"`
	Operation json.RawMessage `json:"operation"`
	Admission admission.Proof `json:"admission"`
}

type controlResponseWire struct {
	Schema          string        `json:"schema"`
	Network         [32]byte      `json:"network"`
	Nonce           [32]byte      `json:"nonce"`
	Deadline        int64         `json:"deadline"`
	OperationDigest [32]byte      `json:"operation_digest"`
	Result          controlResult `json:"result"`
}

type controlBinding struct {
	network  [32]byte
	nonce    [32]byte
	deadline int64
}

type controlRequestValue struct {
	submission authority.Submission
	binding    controlBinding
	admission  admission.Proof
}

func controlRequest(submission authority.Submission, binding controlBinding, admission admission.Proof) ([]byte, error) {
	if binding.network == [32]byte{} || binding.nonce == [32]byte{} || binding.deadline <= 0 ||
		submission.Digest() == [32]byte{} || admission.Challenge.OperationDigest != submission.Digest() {
		return nil, errors.New("private naming control request is invalid")
	}
	raw, err := json.Marshal(controlRequestWire{Schema: controlSchema, Network: binding.network, Nonce: binding.nonce,
		Deadline: binding.deadline, Operation: json.RawMessage(submission.Canonical()), Admission: admission})
	if err != nil {
		return nil, err
	}
	return padMessage(raw)
}

func decodeControlRequest(raw []byte) (controlRequestValue, error) {
	payload, err := unpadMessage(raw)
	if err != nil {
		return controlRequestValue{}, err
	}
	var wire controlRequestWire
	if err := decodeControlJSON(payload, &wire); err != nil || wire.Schema != controlSchema ||
		wire.Network == [32]byte{} || wire.Nonce == [32]byte{} || wire.Deadline <= 0 {
		return controlRequestValue{}, errors.New("private naming control request is invalid")
	}
	submission, err := authority.OpenSubmission(wire.Operation)
	if err != nil {
		return controlRequestValue{}, errors.New("private naming control request is invalid")
	}
	return controlRequestValue{submission: submission, binding: controlBinding{network: wire.Network,
		nonce: wire.Nonce, deadline: wire.Deadline}, admission: wire.Admission}, nil
}

func controlResponse(binding controlBinding, digest [32]byte, result controlResult) ([]byte, error) {
	if binding.network == [32]byte{} || binding.nonce == [32]byte{} || binding.deadline <= 0 ||
		digest == [32]byte{} || !validControlResult(result) {
		return nil, errors.New("private naming control result is invalid")
	}
	raw, err := json.Marshal(controlResponseWire{Schema: controlSchema, Network: binding.network,
		Nonce: binding.nonce, Deadline: binding.deadline, OperationDigest: digest, Result: result})
	if err != nil {
		return nil, err
	}
	return padMessage(raw)
}

func decodeControlResponse(raw []byte) (controlResponseWire, error) {
	payload, err := unpadMessage(raw)
	if err != nil {
		return controlResponseWire{}, err
	}
	var wire controlResponseWire
	if err := decodeControlJSON(payload, &wire); err != nil || wire.Schema != controlSchema ||
		wire.Network == [32]byte{} || wire.Nonce == [32]byte{} || wire.Deadline <= 0 ||
		wire.OperationDigest == [32]byte{} || !validControlResult(wire.Result) {
		return controlResponseWire{}, errors.New("private naming control response is invalid")
	}
	return wire, nil
}

func decodeControlJSON(raw []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("private naming control JSON has trailing input")
	}
	canonical, err := json.Marshal(value)
	if err != nil || !bytes.Equal(canonical, raw) {
		return errors.New("private naming control JSON is non-canonical")
	}
	return nil
}

func validControlResult(result controlResult) bool {
	return result.Class == "submitted" || result.Class == "denied"
}
